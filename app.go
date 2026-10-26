package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"obox-app/buildinfo"
	"obox-app/internal/app_actions"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/obox"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/update"
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
	restart        atomic.Bool
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

type ServerInfo struct {
	IPAddress string `json:"ipAddress"`
	Port      int    `json:"port"`
	Running   bool   `json:"running"`
}

type SystemInfo struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"goVersion"`
	Hostname  string `json:"hostname"`
}

type BuildInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
	Commit    string `json:"commit"`
	Dev       bool   `json:"dev"`
}

type Settings struct {
	AutoStart              bool `json:"autoStart"`
	NetworkPrintingEnabled bool `json:"networkPrintingEnabled"`
	DebugMode              bool `json:"debugMode"`
}

type AppVariable struct {
	AppID    string     `json:"appId"`
	Server   ServerInfo `json:"server"`
	System   SystemInfo `json:"system"`
	Build    BuildInfo  `json:"build"`
	Settings Settings   `json:"settings"`
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

	cfg, err := config.NewManager()
	if err != nil {
		logger.Fatalf("Config initialization failed: %v", err)
	}

	if err := cfg.Load(); err != nil {
		logger.Warnf("Config load warning: %v", err)
	}
	logger.Debugf("Config loaded from %s", cfg.Path())
	a.config = cfg
	logger.SetDebugMode(cfg.IsDebugMode())

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
		a.events.Emit(a.ctx, "odoo:status_changed", status)
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
	a.events.Emit(a.ctx, "app:variables_changed", a.AppVariable())
}

func (a *App) AppVariable() AppVariable {
	debugMode := a.config.IsDebugMode()
	logger.SetDebugMode(debugMode)
	networkEnabled := a.config.IsNetworkPrintingEnabled()
	hostname, _ := os.Hostname()
	return AppVariable{
		AppID: a.config.GetAppID(),
		Server: ServerInfo{
			IPAddress: util.LocalAddr(a.webserver.Port, networkEnabled),
			Port:      a.webserver.Port,
			Running:   a.webserver.Running(),
		},
		System: SystemInfo{
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			GoVersion: runtime.Version(),
			Hostname:  hostname,
		},
		Build: BuildInfo{
			Version:   buildinfo.Version,
			BuildTime: buildinfo.BuildTime,
			Commit:    buildinfo.Commit,
			Dev:       a.ctx != nil && wailsruntime.Environment(a.ctx).BuildType == "dev",
		},
		Settings: Settings{
			AutoStart:              a.autoStart.IsEnabled(),
			NetworkPrintingEnabled: networkEnabled,
			DebugMode:              debugMode,
		},
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
	zipName := fmt.Sprintf("obox-app-logs-%s-%s.zip", a.config.GetAppID(), time.Now().Format("2006-01-02"))
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

	result, err := a.dialogs.Message(a.ctx, wailsruntime.MessageDialogOptions{
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

func (a *App) MarkUpdateSeen(tag string) error {
	return a.config.SetLastSeenUpdate(tag)
}

func (a *App) ToggleDebugMode() error {
	if err := a.config.ToggleDebugMode(); err != nil {
		return err
	}
	logger.SetDebugMode(a.config.IsDebugMode())

	a.notifyAppVariablesChanged()
	return nil
}

func (a *App) CheckForUpdate() update.Info {
	info, err := update.Check(a.ctx)
	if err != nil {
		logger.Errorf("Update check failed: %v", err)
	}
	info.Seen = a.config.LastSeenUpdate() != "" && a.config.LastSeenUpdate() == info.LatestVersion
	return info
}

func (a *App) notifyUpdateProgress(p update.Progress) {
	if a.ctx != nil {
		a.events.Emit(a.ctx, "update-progress", p)
	}
}

func (a *App) DownloadUpdate() (string, error) {
	staged, err := update.Download(context.Background(), func(p update.Progress) {
		a.notifyUpdateProgress(p)
	})
	if err != nil {
		logger.Errorf("Failed to download update: %v", err)
		return "", err
	}

	a.notifyUpdateProgress(update.Progress{State: update.StateInstalling, Percent: 100})
	go func() {
		time.Sleep(5 * time.Second)
		logger.Debugf("Applying update")
		if err := a.ApplyUpdate(); err != nil {
			logger.Errorf("Failed to auto-apply update: %v", err)
			a.notifyUpdateProgress(update.Progress{State: update.StateFailed, Error: err.Error()})
		}
	}()

	return staged, nil
}

func (a *App) confirmQuit() bool {
	if a.restart.Load() {
		return true
	}

	result, err := a.dialogs.Message(a.ctx, wailsruntime.MessageDialogOptions{
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

func (a *App) ApplyUpdate() error {
	u := update.GetUpdaterInstance()
	a.restart.Store(true)

	if err := u.Apply(); err != nil {
		a.restart.Store(false)
		logger.Errorf("Failed to apply update: %v", err)
		return err
	}

	if update.PlatformNeedsQuitAfterApply() {
		if a.webserver != nil && a.webserver.Running() {
			_ = a.webserver.Stop()
		}

		if a.ctx != nil {
			wailsruntime.Quit(a.ctx)
		}
	} else {
		a.restart.Store(false)
		a.notifyUpdateProgress(update.Progress{State: update.StateInstalled})
	}

	return nil
}

func (a *App) CancelUpdate() {
	a.restart.Store(false)
	update.GetUpdaterInstance().Reset()
	if a.ctx != nil {
		a.events.Emit(a.ctx, "update-progress", update.Progress{
			State: update.StateUpdateAvailable,
		})
	}
}
