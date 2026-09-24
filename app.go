package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"obox-app/internal/app_actions"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/obox"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/util"

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
	obox           *obox.Manager
	autoStart      *autostart.App
	dialogs        dialoger
	events         emitter
}

// dlg returns the dialog backend, defaulting to the Wails runtime so an App
// built as a bare struct literal still behaves correctly.
func (a *App) dlg() dialoger {
	if a.dialogs == nil {
		return runtimeDialogs{}
	}
	return a.dialogs
}

// ev returns the event emitter backend, defaulting to the Wails runtime so an App
// built as a bare struct literal still behaves correctly.
func (a *App) ev() emitter {
	if a.events == nil {
		return runtimeEvents{}
	}
	return a.events
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

type AppVariable struct {
	AppID                  string `json:"appId"`
	IPAddress              string `json:"ipAddress"`
	ServerRunning          bool   `json:"serverRunning"`
	OS                     string `json:"os"`
	NetworkPrintingEnabled bool   `json:"networkPrintingEnabled"`
	AutoStart              bool   `json:"autoStart"`
}

func NewApp() *App {
	a := &App{}

	a.autoStart = &autostart.App{
		Name:        "obox-app",
		DisplayName: "Obox App",
		Exec:        []string{os.Args[0]},
	}
	a.dialogs = runtimeDialogs{}
	a.events = runtimeEvents{}

	cfg, err := config.NewManager()
	if err != nil {
		logger.Fatalf("Config initialization failed: %v", err)
	}

	if err := cfg.Load(); err != nil {
		logger.Warnf("Config load warning: %v", err)
	}
	logger.Debugf("Config loaded from %s", cfg.Path())
	a.config = cfg

	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Debugf("Application startup")

	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	a.printerManager = printer.NewManager(a.config)
	a.obox = obox.NewManager(port, a.config, app_actions.OboxActionHandler(a.printerManager))
	a.webserver = server.New(port, a.printerManager, a.obox)
	a.obox.SetOnStatusChange(func(status obox.ConnectionStatus) {
		a.ev().Emit(a.ctx, "odoo:status_changed", status)
	})
}

func (a *App) shutdown(ctx context.Context) {
	logger.Infof("Stopping Obox App server")

	if a.obox != nil {
		a.obox.Stop()
	}

	if err := a.webserver.Stop(); err != nil {
		logger.Errorf("Server stop error: %v", err)
	}
}

func (a *App) notifyAppVariablesChanged() {
	a.ev().Emit(a.ctx, "app:variables_changed", a.AppVariable())
}

func (a *App) AppVariable() AppVariable {
	networkEnabled := a.config.IsNetworkPrintingEnabled()
	return AppVariable{
		AppID:                  a.config.GetAppID(),
		IPAddress:              util.LocalAddr(a.webserver.Port, networkEnabled),
		OS:                     runtime.GOOS,
		ServerRunning:          a.webserver.Running(),
		AutoStart:              a.autoStart.IsEnabled(),
		NetworkPrintingEnabled: networkEnabled,
	}
}

func (a *App) Printers() printer.DiscoveryResult {
	logger.Debug("Collecting printer status")
	result := a.printerManager.DiscoverAllPrinters()
	for i := range result.Printers {
		result.Printers[i].Ip = util.GetPrinterUrl(a.webserver.Port, a.config.IsNetworkPrintingEnabled(), result.Printers[i].Id)
	}
	return result
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
	zipName := fmt.Sprintf("obox-app-logs-%s.zip",
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

func (a *App) ToggleAutoStart() error {
	logger.Info("Toggling autostart")

	if a.autoStart.IsEnabled() {
		if err := a.autoStart.Disable(); err != nil {
			return err
		}
	} else if runtime.GOOS == "linux" {
		if err := util.EnableLinuxAutostart(); err != nil {
			return err
		}
	} else {
		if err := a.autoStart.Enable(); err != nil {
			return err
		}
	}

	a.notifyAppVariablesChanged()
	return nil
}

func (a *App) CheckOdooStatus() obox.ConnectionStatus {
	logger.Debugf("checking Odoo status")

	if a.obox != nil {
		return a.obox.ConnectionStatus()
	}

	return obox.ConnectionStatus{
		WebsocketStatus: obox.StateDisconnected,
		LanStatus:       obox.StateDisconnected,
	}
}

func (a *App) ConfirmDisconnectOdoo() (bool, error) {
	logger.Debugf("Confirm Disconnect Odoo requested")

	if a.obox == nil {
		return false, fmt.Errorf("Odoo connection is not initialized")
	}

	result, err := a.dlg().Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Disconnect Odoo",
		Message:       "Are you sure you want to disconnect and remove the Odoo database connection?",
		Buttons:       []string{"Cancel", "Disconnect"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return false, fmt.Errorf("failed to show confirmation dialog: %w", err)
	}

	// In Windows & Linux, the Button field is not ovverride by Button struct
	// so even here we used "Cancel", "Confirm" it shows {No, Yes}
	if result != "Disconnect" && result != "Confirm" && result != "Yes" {
		return false, nil
	}

	a.obox.Disconnect()
	return true, nil
}

func (a *App) ToggleNetworkPrinting() error {
	logger.Infof("Toggling network printing %v", !a.config.IsNetworkPrintingEnabled())
	if err := a.config.ToggleNetworkPrinting(); err != nil {
		logger.Errorf("Failed to toggle network printing: %v", err)
		return err
	}
	a.notifyAppVariablesChanged()
	return nil
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

func (a *App) ConfirmQuit() bool {
	result, err := a.dlg().Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Quit Obox App",
		Message:       "Stopping the Obox App will prevent POS from printing receipts.\n\nAre you sure you want to quit?",
		Buttons:       []string{"Cancel", "Quit"},
		DefaultButton: "Cancel",
	})

	if err != nil {
		logger.Errorf("Failed to show quit dialog: %v", err)
		return false
	}

	// linux doesn't use Buttons overrides and uses No | Yes for question dialog
	if result != "Yes" && result != "Quit" {
		return false
	}

	logger.Debug("Confirmed quit action")
	return true
}
