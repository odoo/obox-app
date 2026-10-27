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

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// dialoger abstracts UI dialogs for testing and decoupling.
type dialoger interface {
	Error(title, message string)
	Question(title, message, confirmLabel, cancelLabel string) bool
	SaveFile(title, filename, filterDisplayName, filterPattern string) (string, error)
}

// emitter abstracts Wails event broadcasting.
type emitter interface {
	Emit(name string, data ...any) bool
}

// wailsDialogs implements dialoger using the Wails v3 Dialog manager.
type wailsDialogs struct {
	app *application.App
}

func (w *wailsDialogs) Error(title, message string) {
	if w.app == nil || w.app.Dialog == nil {
		logger.Errorf("Error dialog [no app]: %s — %s", title, message)
		return
	}
	w.app.Dialog.Error().SetTitle(title).SetMessage(message).Show()
}

func (w *wailsDialogs) Question(title, message, confirmLabel, cancelLabel string) bool {
	if w.app == nil || w.app.Dialog == nil {
		return true
	}
	ch := make(chan bool, 1)
	dialog := w.app.Dialog.Question().
		SetTitle(title).
		SetMessage(message)

	dialog.AddButton(cancelLabel).SetAsCancel().OnClick(func() {
		ch <- false
	})
	dialog.AddButton(confirmLabel).SetAsDefault().OnClick(func() {
		ch <- true
	})
	dialog.Show()

	select {
	case confirmed := <-ch:
		return confirmed
	case <-time.After(5 * time.Minute):
		return false
	}
}

func (w *wailsDialogs) SaveFile(title, filename, filterDisplayName, filterPattern string) (string, error) {
	if w.app == nil || w.app.Dialog == nil {
		return "", nil
	}
	dlg := w.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    title,
		Filename: filename,
	})
	if filterDisplayName != "" && filterPattern != "" {
		dlg.AddFilter(filterDisplayName, filterPattern)
	}
	return dlg.PromptForSingleSelection()
}

// wailsEvents implements emitter using Wails v3 Event manager.
type wailsEvents struct {
	app *application.App
}

func (w *wailsEvents) Emit(name string, data ...any) bool {
	if w.app == nil || w.app.Event == nil {
		return false
	}
	return w.app.Event.Emit(name, data...)
}

// App is the Wails service that exposes all backend methods to the frontend.
// In Wails v3, public methods on a service struct are automatically bound.
type App struct {
	app                 *application.App
	window              *application.WebviewWindow
	notificationService *notifications.NotificationService
	dialogs             dialoger
	events              emitter
	webserver           *server.Server
	config              *config.Manager
	printerManager      *printer.Manager
	oboxMgr             *obox.Manager
	restart             atomic.Bool

	// trayStatusItem is the dynamic "Odoo: …" menu item in the system tray.
	trayStatusItem *application.MenuItem
}

// ── Internal setup ─────────────────────────────────────────────────────────

func NewApp() *App {
	a := &App{
		dialogs: &wailsDialogs{},
		events:  &wailsEvents{},
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

// setApp is called from main.go after application.New() to give the service
// a handle to the application (for events, dialogs, etc.).
func (a *App) setApp(app *application.App) {
	a.app = app
	a.dialogs = &wailsDialogs{app: app}
	a.events = &wailsEvents{app: app}
}

// setWindow is called from main.go after window creation.
func (a *App) setWindow(w *application.WebviewWindow) {
	a.window = w
}

// setNotifications connects the notification service to App.
func (a *App) setNotifications(ns *notifications.NotificationService) {
	a.notificationService = ns
}

// setTrayStatusItem stores the reference to the dynamic tray status menu item.
func (a *App) setTrayStatusItem(item *application.MenuItem) {
	a.trayStatusItem = item
}

// startup is invoked once the application event loop is running (hooked via
// OnApplicationEvent in main.go).
func (a *App) startup() {
	logger.Debugf("Application startup")

	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	a.printerManager = printer.NewManager(a.config)
	a.oboxMgr = obox.NewManager(port, a.config, app_actions.OboxActionHandler(a.printerManager))
	a.webserver = server.New(port, a.printerManager, a.oboxMgr)

	a.oboxMgr.SetOnStatusChange(func(status obox.ConnectionStatus) {
		if a.events != nil {
			a.events.Emit("odoo:status_changed", status)
		}
		a.updateTrayStatus(status)
	})
}

// shutdown cleanly stops the backend servers on app exit.
func (a *App) shutdown() {
	logger.Infof("Application shutdown")
	if a.oboxMgr != nil {
		a.oboxMgr.Stop()
	}
	if a.webserver != nil && a.webserver.Running() {
		_ = a.webserver.Stop()
	}
}

// updateTrayStatus updates the dynamic status item label in the system tray.
func (a *App) updateTrayStatus(status obox.ConnectionStatus) {
	if a.trayStatusItem == nil {
		return
	}
	label := "Odoo: disconnected"
	if status.WebsocketStatus == obox.StateConnected || status.LanStatus == obox.StateConnected {
		label = "Odoo: connected ✓"
	} else if status.WebsocketStatus == obox.StateConnecting || status.LanStatus == obox.StateConnecting {
		label = "Odoo: connecting…"
	}
	a.trayStatusItem.SetLabel(label)
}

// showError shows a native error dialog.
func (a *App) showError(title, message string) {
	if a.dialogs != nil {
		a.dialogs.Error(title, message)
	}
}

// ── Exported types (mirrored to frontend via wails3 generate bindings) ─────

type ServerInfo struct {
	IPAddress string `json:"ipAddress"`
	Port      int    `json:"port"`
	Running   bool   `json:"running"`
}

type SystemInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
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

// ── Public service methods (auto-bound by Wails v3) ────────────────────────

func (a *App) AppVariable() AppVariable {
	debugMode := a.config.IsDebugMode()
	logger.SetDebugMode(debugMode)
	networkEnabled := a.config.IsNetworkPrintingEnabled()
	hostname, _ := os.Hostname()

	serverInfo := ServerInfo{}
	if a.webserver != nil {
		serverInfo = ServerInfo{
			IPAddress: util.LocalAddr(a.webserver.Port, networkEnabled),
			Port:      a.webserver.Port,
			Running:   a.webserver.Running(),
		}
	}

	autoStart := a.config.IsAutoStartEnabled()
	if a.app != nil && a.app.Autostart != nil {
		if enabled, err := a.app.Autostart.IsEnabled(); err == nil {
			autoStart = enabled
		}
	}

	return AppVariable{
		AppID:  a.config.GetAppID(),
		Server: serverInfo,
		System: SystemInfo{
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
			Hostname: hostname,
		},
		Build: BuildInfo{
			Version:   buildinfo.Version,
			BuildTime: buildinfo.BuildTime,
			Commit:    buildinfo.Commit,
			Dev:       buildinfo.Version == "dev",
		},
		Settings: Settings{
			AutoStart:              autoStart,
			NetworkPrintingEnabled: networkEnabled,
			DebugMode:              debugMode,
		},
	}
}

func (a *App) notifyAppVariablesChanged() {
	if a.events != nil {
		a.events.Emit("app:variables_changed", a.AppVariable())
	}
}

func (a *App) Printers() printer.DiscoveryResult {
	logger.Debug("Collecting printer status")
	result := a.printerManager.DiscoverAllPrinters()
	for i := range result.Printers {
		port := 0
		if a.webserver != nil {
			port = a.webserver.Port
		}
		result.Printers[i].Ip = util.GetPrinterUrl(port, a.config.IsNetworkPrintingEnabled(), result.Printers[i].Id)
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
	msg := fmt.Sprintf("Are you sure you want to remove the printer at %s?", ip)
	confirmed := false
	if a.dialogs != nil {
		confirmed = a.dialogs.Question("Remove Printer", msg, "Confirm", "Cancel")
	}
	if confirmed {
		if err := a.config.RemoveLANPrinter(ip); err != nil {
			return false, fmt.Errorf("failed to remove LAN printer: %w", err)
		}
		return true, nil
	}
	logger.Infof("Remove LAN printer cancelled: %s", ip)
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
	if a.dialogs == nil {
		return
	}
	savePath, err := a.dialogs.SaveFile("Save Archive", zipName, "Zip Archives (*.zip)", "*.zip")
	if err != nil {
		logger.Errorf("Save dialog failed: %v", err)
		a.showError("Download Logs Failed", err.Error())
		return
	}
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

// ToggleAutoStart toggles start-at-login using the Wails v3 built-in mechanism.
func (a *App) ToggleAutoStart() error {
	logger.Info("Toggling autostart")
	if a.app != nil && a.app.Autostart != nil {
		enabled, err := a.app.Autostart.IsEnabled()
		if err != nil {
			logger.Warnf("Could not check autostart status, falling back to config: %v", err)
			enabled = a.config.IsAutoStartEnabled()
		}
		var targetErr error
		if enabled {
			targetErr = a.app.Autostart.Disable()
		} else {
			targetErr = a.app.Autostart.Enable()
		}
		if targetErr != nil {
			logger.Errorf("Failed to toggle start at login: %v", targetErr)
			return targetErr
		}
		if err := a.config.SetAutoStartEnabled(!enabled); err != nil {
			logger.Errorf("Failed to persist autostart setting: %v", err)
			return err
		}
	} else {
		current := a.config.IsAutoStartEnabled()
		if err := a.config.SetAutoStartEnabled(!current); err != nil {
			return err
		}
	}
	a.notifyAppVariablesChanged()
	return nil
}

func (a *App) CheckOdooStatus() obox.ConnectionStatus {
	logger.Debugf("checking Odoo status")
	if a.oboxMgr != nil {
		return a.oboxMgr.ConnectionStatus()
	}
	return obox.ConnectionStatus{
		WebsocketStatus: obox.StateDisconnected,
		LanStatus:       obox.StateDisconnected,
	}
}

func (a *App) ConfirmDisconnectOdoo() (bool, error) {
	logger.Debugf("Confirm Disconnect Odoo requested")
	if a.oboxMgr == nil {
		return false, fmt.Errorf("Odoo connection is not initialized")
	}
	confirmed := false
	if a.dialogs != nil {
		confirmed = a.dialogs.Question(
			"Disconnect Odoo",
			"Are you sure you want to disconnect and remove the Odoo database connection?",
			"Disconnect",
			"Cancel",
		)
	}
	if !confirmed {
		return false, nil
	}
	a.oboxMgr.Disconnect()
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
	info, err := update.Check(context.Background())
	if err != nil {
		logger.Errorf("Update check failed: %v", err)
	}
	info.Seen = a.config.LastSeenUpdate() != "" && a.config.LastSeenUpdate() == info.LatestVersion
	if !info.Seen && info.State == update.StateUpdateAvailable {
		a.sendUpdateNotification(info.LatestVersion)
	}
	return info
}

// sendUpdateNotification fires an OS notification when a new version is found.
func (a *App) sendUpdateNotification(version string) {
	if a.notificationService != nil {
		_ = a.notificationService.SendNotification(notifications.NotificationOptions{
			ID:    "obox-update",
			Title: "Update Available",
			Body:  fmt.Sprintf("Obox App %s is ready to install.", version),
		})
	}
}

func (a *App) notifyUpdateProgress(p update.Progress) {
	if a.events != nil {
		a.events.Emit("update-progress", p)
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
	msg := "Stopping the proxy will prevent POS from printing receipts.\n\nAre you sure you want to quit?"
	if a.dialogs != nil && !a.dialogs.Question("Quit Obox App", msg, "Quit", "Cancel") {
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

		if a.app != nil {
			a.app.Quit()
		}
	} else {
		a.restart.Store(false)
		a.notifyUpdateProgress(update.Progress{State: update.StateInstalled})
		if a.notificationService != nil {
			_ = a.notificationService.SendNotification(notifications.NotificationOptions{
				ID:    "obox-update-installed",
				Title: "Update Installed",
				Body:  "Obox App has been updated. Restart to apply the new version.",
			})
		}
	}

	return nil
}

func (a *App) CancelUpdate() {
	a.restart.Store(false)
	update.GetUpdaterInstance().Reset()
	if a.events != nil {
		a.events.Emit("update-progress", update.Progress{
			State: update.StateUpdateAvailable,
		})
	}
}

// ── Crash reporter ─────────────────────────────────────────────────────────

// ReportCrash is called by the frontend when it catches an unhandled JS error
// or React render failure. It logs the crash and shows an error dialog so the
// user knows to submit logs.
func (a *App) ReportCrash(message string, stack string) {
	logger.Errorf("Frontend crash reported — message: %s\nStack:\n%s", message, stack)
	if a.dialogs != nil {
		a.dialogs.Error(
			"Application Error",
			"An unexpected error occurred.\n\nPlease use 'Download Logs' to export logs and report this issue.\n\n"+message,
		)
	}
}
