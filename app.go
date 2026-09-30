package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"epos-proxy/internal/bluetooth"
	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/server"
	"epos-proxy/internal/util"

	autostart "github.com/emersion/go-autostart"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// dialoger abstracts the Wails runtime dialog calls. Production code uses
// runtimeDialogs; tests substitute a fake so the dialog-driven code paths can
// be exercised without a live Wails context.
type dialoger interface {
	Message(ctx context.Context, opts wailsruntime.MessageDialogOptions) (string, error)
	SaveFile(ctx context.Context, opts wailsruntime.SaveDialogOptions) (string, error)
}

// runtimeDialogs forwards to the real Wails runtime.
type runtimeDialogs struct{}

func (runtimeDialogs) Message(ctx context.Context, opts wailsruntime.MessageDialogOptions) (string, error) {
	return wailsruntime.MessageDialog(ctx, opts)
}

func (runtimeDialogs) SaveFile(ctx context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
	return wailsruntime.SaveFileDialog(ctx, opts)
}

// App struct
type App struct {
	ctx            context.Context
	webserver      *server.Server
	config         *config.Manager
	printerManager *printer.Manager
	autoStart      *autostart.App
	dialogs        dialoger
}

// dlg returns the dialog backend, defaulting to the Wails runtime so an App
// built as a bare struct literal still behaves correctly.
func (a *App) dlg() dialoger {
	if a.dialogs == nil {
		return runtimeDialogs{}
	}
	return a.dialogs
}

// showError surfaces an error to the user and logs any failure to do so.
func (a *App) showError(title, message string) {
	if _, err := a.dlg().Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.ErrorDialog,
		Title:   title,
		Message: message,
	}); err != nil {
		logger.Errorf("Failed to show error dialog %q: %v", title, err)
	}
}

type Printer struct {
	Name           string           `json:"name"`
	Ip             string           `json:"ip"`
	Id             string           `json:"id"`
	ConnectionType printer.ConnKind `json:"connectionType"`
	LANIp          string           `json:"lanIp,omitempty"`
	BTMac          string           `json:"btMac,omitempty"`
	Online         bool             `json:"online"`
	Type           string           `json:"type"`
}

type UnavailablePrinter struct {
	Name           string           `json:"name"`
	ErrorMsg       string           `json:"errorMsg"`
	ConnectionType printer.ConnKind `json:"connectionType"`
	LANIp          string           `json:"lanIp,omitempty"`
	BTMac          string           `json:"btMac,omitempty"`
}

type AppVariable struct {
	ServerRunning bool   `json:"serverRunning"`
	Os            string `json:"os"`
}

type Printers struct {
	ErrorMsg            string               `json:"errorMsg"`
	Printers            []Printer            `json:"printers"`
	UnavailablePrinters []UnavailablePrinter `json:"unavailablePrinters"`
}

func NewApp() *App {
	a := &App{}

	a.autoStart = &autostart.App{
		Name:        "epos-proxy",
		DisplayName: "ePOS Proxy",
		Exec:        []string{os.Args[0]},
	}
	a.dialogs = runtimeDialogs{}

	cfg, err := config.NewManager()
	if err != nil {
		logger.Fatalf("Config initialization failed: %v", err)
	}

	if err := cfg.Load(); err != nil {
		logger.Warnf("Config load warning: %v", err)
	}

	a.config = cfg
	a.printerManager = printer.NewManager(cfg)

	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Debugf("Application startup")
	logger.Debugf("Config loaded from %s", a.config.Path())

	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	a.webserver = server.New(port, a.printerManager)
}

func (a *App) shutdown(ctx context.Context) {
	logger.Infof("Stopping proxy server")

	if err := a.webserver.Stop(); err != nil {
		logger.Errorf("Server stop error: %v", err)
	}
}

func (a *App) AppVariable() AppVariable {
	return AppVariable{
		Os:            runtime.GOOS,
		ServerRunning: a.webserver.Running(),
	}
}

func (a *App) GetPrinterUrl(id string) string {
	url := fmt.Sprintf("%s:%d/p/%s", util.GetLocalIP(a.config.IsNetworkPrintingEnabled()), a.webserver.Port, id)
	logger.Debugf("Generated printer endpoint: %s", url)
	return url
}

func (a *App) Printers() Printers {

	logger.Debug("Collecting printer status")

	printers := make([]Printer, 0)
	unavailablePrinters := make([]UnavailablePrinter, 0)

	printerInfos, err := printer.ListUSBPrinters()
	errorMsg := ""
	if err == nil {

		logger.Debugf("Detected %d available USB printers", len(printerInfos.Available))

		for _, info := range printerInfos.Available {
			printers = append(printers, Printer{
				Id:             info.Id,
				Name:           info.Name,
				Ip:             a.GetPrinterUrl(info.Id),
				ConnectionType: printer.ConnKindUSB,
				Online:         true,
				Type:           string(info.Type),
			})
		}

		for _, info := range printerInfos.Unavailable {
			unavailablePrinters = append(unavailablePrinters, UnavailablePrinter{
				Name:           info.Name,
				ErrorMsg:       info.Error,
				ConnectionType: printer.ConnKindUSB,
			})

			logger.Warnf("USB printer unavailable: %s (%s)", info.Name, info.Error)
		}
	} else {
		errorMsg = err.Error()
		logger.Errorf("USB printer detection failed: %v", err)
	}

	lanPrinters := a.printerManager.ListLANPrinters()
	for _, info := range lanPrinters {
		printers = append(printers, Printer{
			Id:             info.Id,
			Name:           fmt.Sprintf("Network - %s", info.IP),
			Ip:             a.GetPrinterUrl(info.Id),
			ConnectionType: printer.ConnKindLAN,
			LANIp:          info.IP,
			Type:           string(printer.TypeReceipt),
		})
	}

	// Bluetooth printers from config
	btPrinters := a.printerManager.ListBluetoothPrinters()
	for _, btCfg := range btPrinters {
		printers = append(printers, Printer{
			Id:             btCfg.Id,
			Name:           btCfg.Name,
			Ip:             a.GetPrinterUrl(btCfg.Id),
			ConnectionType: printer.ConnKindBT,
			BTMac:          btCfg.Address,
			Type:           string(printer.TypeReceipt),
		})
	}

	return Printers{
		Printers:            printers,
		UnavailablePrinters: unavailablePrinters,
		ErrorMsg:            errorMsg,
	}
}

func (a *App) AddPrinter(p printer.RawPrinter) error {
	switch p.ConnectionType {
	case printer.ConnKindLAN:
		return a.printerManager.AddLANPrinter(p)
	case printer.ConnKindBT:
		return a.printerManager.AddBluetoothPrinter(p)
	default:
		return fmt.Errorf("unsupported connection type: %s", p.ConnectionType)
	}
}

func (a *App) ConfirmRemoveLANPrinter(ip string) (bool, error) {
	logger.Debugf("Remove LAN printer requested: %s", ip)

	result, err := a.dlg().Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Remove Printer",
		Message:       fmt.Sprintf("Are you sure you want to remove the printer at %s?", ip),
		Buttons:       []string{"Cancel", "Confirm"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return false, fmt.Errorf("failed to show confirmation dialog: %w", err)
	}
	if result == "Confirm" || result == "Yes" {
		if err := a.config.RemoveLANPrinter(ip); err != nil {
			return false, fmt.Errorf("failed to remove LAN printer: %w", err)
		}
		return true, nil
	}
	logger.Infof("Remove LAN printer cancelled, Remove printer dialog result: %s", result)
	return false, nil
}

func (a *App) CheckLANPrinterStatus(ip string) bool {
	logger.Debugf("Checking LAN printer status: %s", ip)
	return printer.CheckLANPrinter(ip) == nil
}

func (a *App) DownloadLogs() {
	logger.Debugf("Download logs requested")
	logDir := logger.LogDirectory()
	zipName := fmt.Sprintf("epos-proxy-logs-%s.zip",
		time.Now().Format("2006-01-02"))
	logger.Debugf("Creating logs archive: %s", zipName)
	savePath, err := a.dlg().SaveFile(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save Archive",
		DefaultFilename: zipName,
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Zip Archives (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
	if err != nil {
		logger.Errorf("Save dialog failed: %v", err)
		a.showError("Download Logs Failed", err.Error())
		return
	}

	// An empty path means the user dismissed the save dialog.
	if savePath == "" {
		logger.Infof("Download logs cancelled by user")
		return
	}

	if err := util.ZipLogs(logDir, savePath); err != nil {
		logger.Errorf("Log export failed: %v", err)
		a.showError("Download Logs Failed", err.Error())
		return
	}
	logger.Infof("Logs successfully exported to: %s", savePath)
}

func (a *App) IsAutostartEnabled() bool {
	return a.autoStart.IsEnabled()
}

func (a *App) EnableAutostart() error {
	logger.Info("Enabling autostart")

	if runtime.GOOS == "linux" {
		return util.EnableLinuxAutostart()
	}

	if !a.autoStart.IsEnabled() {
		return a.autoStart.Enable()
	}

	return nil
}

func (a *App) DisableAutostart() error {
	logger.Info("Disabling autostart")

	if a.autoStart.IsEnabled() {
		return a.autoStart.Disable()
	}

	return nil
}

func (a *App) SetNetworkPrintingEnabled(enabled bool) error {
	logger.Infof("Setting network printing enabled: %v", enabled)
	return a.config.SetNetworkPrintingEnabled(enabled)
}

func (a *App) IsNetworkPrintingEnabled() bool {
	if a.config == nil {
		return false
	}
	return a.config.IsNetworkPrintingEnabled()
}

type TroubleshootInfo struct {
	ActiveFirewall string `json:"activeFirewall"`
	FirewallZone   string `json:"firewallZone"`
	Port           int    `json:"port"`
	Subnet         string `json:"subnet"`
	LocalIP        string `json:"localIp"`
	ExecPath       string `json:"execPath"`
}

func (a *App) GetTroubleshootInfo() TroubleshootInfo {
	netInfo := util.GetNetworkInfo()
	execPath, _ := os.Executable()
	return TroubleshootInfo{
		ActiveFirewall: netInfo.ActiveFirewall,
		FirewallZone:   netInfo.Zone,
		Port:           a.config.GetPort(),
		Subnet:         netInfo.Subnet,
		LocalIP:        netInfo.IP,
		ExecPath:       execPath,
	}
}

func (a *App) ScanBluetoothPrinters() ([]bluetooth.BluetoothPrinterInfo, error) {
	logger.Debug("Scanning for Bluetooth devices")
	devices, err := bluetooth.Scan()
	if err != nil {
		logger.Errorf("Bluetooth scan failed: %v", err)
		return nil, err
	}
	return devices, nil
}

func (a *App) CheckBluetoothDependencies() []bluetooth.DependencyStatus {
	return bluetooth.CheckDependencies()
}

func (a *App) ConfirmRemoveBluetoothPrinter(address string) (bool, error) {
	logger.Debugf("Remove Bluetooth printer requested: %s", address)

	result, err := a.dlg().Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Remove Printer",
		Message:       fmt.Sprintf("Are you sure you want to remove the Bluetooth printer %s?", address),
		Buttons:       []string{"Cancel", "Confirm"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		logger.Errorf("failed to show confirmation dialog: %v", err)
		return false, fmt.Errorf("failed to show confirmation dialog: %w", err)
	}
	if result == "Confirm" || result == "Yes" {
		if err := a.printerManager.RemoveBluetoothPrinter(address); err != nil {
			logger.Errorf("Failed to remove Bluetooth printer: %v", err)
			return false, err
		}
		logger.Debugf("Bluetooth printer removed successfully")
		return true, nil
	}
	logger.Debugf("Remove Bluetooth printer cancelled")
	return false, nil
}

func (a *App) CheckBluetoothPrinterStatus(address string) bool {
	logger.Debugf("Checking Bluetooth printer status: %s", address)
	if err := a.printerManager.CheckBluetoothPrinter(address); err != nil {
		logger.Errorf("Bluetooth printer %s check failed: %v", address, err)
		return false
	}
	return true
}
