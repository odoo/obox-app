package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"obox-app/buildinfo"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/update"
	"obox-app/internal/util"

	autostart "github.com/emersion/go-autostart"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type UpdateInfo = update.Info

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

// emitter abstracts Wails runtime event emissions. Production code uses
// runtimeEvents; tests substitute a fake so event-driven code paths can
// be exercised without a live Wails context.
type emitter interface {
	Emit(ctx context.Context, eventName string, optionalData ...interface{})
}

// runtimeEvents forwards to the real Wails runtime.
type runtimeEvents struct{}

func (runtimeEvents) Emit(ctx context.Context, eventName string, optionalData ...interface{}) {
	if ctx == nil {
		return
	}
	wailsruntime.EventsEmit(ctx, eventName, optionalData...)
}

// App struct
type App struct {
	ctx            context.Context
	webserver      *server.Server
	config         *config.Manager
	printerManager *printer.Manager
	autoStart      *autostart.App
	dialogs        dialoger
	events         emitter
	restart        bool
}

// showError surfaces an error to the user and logs any failure to do so.
func (a *App) showError(title, message string) {
	if _, err := a.dialogs.Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.ErrorDialog,
		Title:   title,
		Message: message,
	}); err != nil {
		logger.Errorf("Failed to show error dialog %q: %v", title, err)
	}
}

type Printer struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Id     string `json:"id"`
	IsLAN  bool   `json:"isLAN"`
	LANIp  string `json:"lanIp,omitempty"`
	Online bool   `json:"online"`
	Type   string `json:"type"`
}

type UnavailablePrinter struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type AppVariable struct {
	ServerRunning bool   `json:"serverRunning"`
	Os            string `json:"os"`
	Version       string `json:"version"`
	BuildTime     string `json:"buildTime"`
	Commit        string `json:"commit"`
	DebugMode     bool   `json:"debugMode"`
	IsDev         bool   `json:"isDev"`
}

type Printers struct {
	ErrorMsg            string               `json:"errorMsg"`
	Printers            []Printer            `json:"printers"`
	UnavailablePrinters []UnavailablePrinter `json:"unavailablePrinters"`
}

func NewApp() *App {
	a := &App{
		dialogs: runtimeDialogs{},
		events:  runtimeEvents{},
	}

	a.autoStart = &autostart.App{
		Name:        "obox-app",
		DisplayName: "Obox App",
		Exec:        []string{os.Args[0]},
	}
	a.printerManager = printer.NewManager()

	cfg, err := config.NewManager()
	if err != nil {
		logger.Fatalf("Config initialization failed: %v", err)
	}

	if err := cfg.Load(); err != nil {
		logger.Warnf("Config load warning: %v", err)
	}

	a.config = cfg
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
	logger.Infof("Stopping Obox App server")

	if err := a.webserver.Stop(); err != nil {
		logger.Errorf("Server stop error: %v", err)
	}
}

func (a *App) AppVariable() AppVariable {
	return AppVariable{
		Os:            runtime.GOOS,
		ServerRunning: a.webserver.Running(),
		Version:       buildinfo.Version,
		BuildTime:     buildinfo.BuildTime,
		Commit:        buildinfo.Commit,
		DebugMode:     a.config.IsDebugMode(),
		IsDev:         a.IsDevMode(),
	}
}

func (a *App) IsDevMode() bool {
	return wailsruntime.Environment(a.ctx).BuildType == "dev"
}

func (a *App) GetPrinterUrl(id string) string {
	url := fmt.Sprintf("%s:%d/p/%s", util.GetLocalIP(a.config.IsNetworkPrintingEnabled()), a.webserver.Port, id)
	logger.Debugf("Generated printer endpoint: %s", url)
	return url
}

func (a *App) notifyAppVariablesChanged() {
	a.events.Emit(a.ctx, "app:variables_changed", a.AppVariable())
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
				Id:     info.Id,
				Name:   info.Name,
				Ip:     a.GetPrinterUrl(info.Id),
				Online: true,
				Type:   string(info.Type),
			})
		}

		for _, info := range printerInfos.Unavailable {
			unavailablePrinters = append(unavailablePrinters, UnavailablePrinter{
				Name:     info.Name,
				ErrorMsg: info.Error,
			})

			logger.Warnf("USB printer unavailable: %s (%s)", info.Name, info.Error)
		}
	} else {
		errorMsg = err.Error()
		logger.Errorf("USB printer detection failed: %v", err)
	}

	lanPrinters := printer.ListLANPrinters(a.config)

	for _, info := range lanPrinters {
		printers = append(printers, Printer{
			Id:    info.Id,
			Name:  fmt.Sprintf("Network - %s", info.IP),
			Ip:    a.GetPrinterUrl(info.Id),
			IsLAN: true,
			LANIp: info.IP,
			Type:  string(printer.TypeReceipt),
		})
	}

	return Printers{
		Printers:            printers,
		UnavailablePrinters: unavailablePrinters,
		ErrorMsg:            errorMsg,
	}
}

func (a *App) AddLANPrinter(ip string) error {
	logger.Debugf("Adding LAN printer: %s", ip)

	ip, err := printer.ValidateIPAddress(ip)
	if err != nil {
		return fmt.Errorf("invalid IP address: %s, error: %v", ip, err)
	}

	if err := printer.CheckLANPrinter(ip); err != nil {
		return fmt.Errorf("LAN printer unreachable: %s, error: %v", ip, err)
	}

	if err := a.config.AddLanEposPrinter(ip); err != nil {
		return fmt.Errorf("failed to save LAN printer: %s, error: %v", ip, err)
	}

	logger.Debugf("LAN printer added successfully: %s", ip)
	return nil
}

func (a *App) ConfirmRemoveLANPrinter(ip string) (bool, error) {
	logger.Debugf("Remove LAN printer requested: %s", ip)

	result, err := a.dialogs.Message(a.ctx, wailsruntime.MessageDialogOptions{
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
	zipName := fmt.Sprintf("obox-app-logs-%s.zip",
		time.Now().Format("2006-01-02"))
	logger.Debugf("Creating logs archive: %s", zipName)
	savePath, err := a.dialogs.SaveFile(a.ctx, wailsruntime.SaveDialogOptions{
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

func (a *App) MarkUpdateSeen(tag string) error {
	return a.config.SetLastSeenUpdate(tag)
}

func (a *App) notifyUpdateAvailable(info *update.Info) {
	a.events.Emit(a.ctx, "update-available", info)
}

func (a *App) CheckForUpdate() UpdateInfo {
	info, err := update.Check(a.ctx)
	info.Seen = a.config.LastSeenUpdate() == info.LatestVersion
	if err != nil {
		logger.Errorf("Update check failed: %v", err)
	}
	return info
}

func (a *App) SetSupportModeEnabled(enabled bool) error {
	logger.SetDebugMode(enabled)
	if err := a.config.SetDebugMode(enabled); err != nil {
		return err
	}
	a.notifyAppVariablesChanged()
	return nil
}

func (a *App) DownloadUpdate() (string, error) {
	staged, err := update.Download(context.Background(), func(p update.Progress) {
		if a.ctx != nil {
			a.events.Emit(a.ctx, "update-progress", p)
		}
	})
	if err != nil {
		return "", err
	}

	if a.ctx != nil {
		a.events.Emit(a.ctx, "update-progress", update.Progress{
			State:   update.StateInstalling,
			Percent: 100,
		})
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		if err := a.ApplyUpdate(); err != nil {
			logger.Errorf("Failed to auto-apply update: %v", err)
			if a.ctx != nil {
				a.events.Emit(a.ctx, "update-progress", update.Progress{
					State: update.StateFailed,
				})
			}
		}
	}()

	return staged, nil
}

func (a *App) ApplyUpdate() error {
	u := update.GetUpdaterInstance()
	a.restart = true

	// Stop background services and release locks before applying
	wasRunning := a.webserver != nil && a.webserver.Running()
	if wasRunning {
		_ = a.webserver.Stop()
	}

	if err := u.Apply(); err != nil {
		a.restart = false
		if wasRunning && a.config != nil {
			if port, portErr := a.config.ResolvePort(); portErr == nil {
				a.webserver = server.New(port, a.printerManager)
			}
		}
		logger.Errorf("Failed to apply update: %v", err)
		return err
	}

	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
	return nil
}

// CancelUpdate aborts an in-progress update download and resets updater state to idle.
func (a *App) CancelUpdate() {
	update.GetUpdaterInstance().Reset()
}
