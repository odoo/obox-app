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
	"github.com/wailsapp/wails/v3/pkg/updater"
)

type App struct {
	app                 *application.App
	notificationService *notifications.NotificationService
	webserver           *server.Server
	config              *config.Manager
	printerManager      *printer.Manager
	obox                *obox.Manager
	restart             atomic.Bool
	updater             update.Updater
	trayStatusItem      *application.MenuItem
}

func NewApp() *App {
	cfg, err := config.NewManager()
	if err != nil {
		logger.Fatalf("Config initialization failed: %v", err)
	}
	if err := cfg.Load(); err != nil {
		logger.Warnf("Config load warning: %v", err)
	}
	logger.Debugf("Config loaded from %s", cfg.Path())
	logger.SetDebugMode(cfg.IsDebugMode())

	return &App{
		updater: update.New(),
		config:  cfg,
	}
}

// ServiceStartup is invoked automatically by Wails v3 when the application starts.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.setApp(application.Get())
	a.startup()
	return nil
}

// ServiceShutdown is invoked automatically by Wails v3 during application shutdown.
func (a *App) ServiceShutdown() error {
	a.shutdown()
	return nil
}

func (a *App) setApp(app *application.App) {
	a.app = app
}

// setNotifications connects the notification service to App.
func (a *App) setNotifications(ns *notifications.NotificationService) {
	a.notificationService = ns
}

// setTrayStatusItem stores the reference to the dynamic tray status menu item.
func (a *App) setTrayStatusItem(item *application.MenuItem) {
	a.trayStatusItem = item
}

// startup is invoked once the application event loop is running.
func (a *App) startup() {
	logger.Debugf("Application startup")

	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	a.printerManager = printer.NewManager(a.config)
	a.obox = obox.NewManager(port, a.config, app_actions.OboxActionHandler(a.printerManager))
	a.webserver = server.New(port, a.printerManager, a.obox)

	a.obox.SetOnStatusChange(func(status obox.ConnectionStatus) {
		a.emit("odoo:status_changed", status)
		a.updateTrayStatus(status)
	})
	a.updateTrayStatus(a.obox.ConnectionStatus())
}

// shutdown cleanly stops the backend servers on app exit.
func (a *App) shutdown() {
	logger.Infof("Application shutdown")
	if a.obox != nil {
		a.obox.Stop()
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

// emit broadcasts an event to the frontend via Wails v3 Event manager.
func (a *App) emit(name string, data ...any) {
	if a.app != nil && a.app.Event != nil {
		a.app.Event.Emit(name, data...)
	}
}

// showError shows a native error dialog.
func (a *App) showError(title, message string) {
	if a.app != nil && a.app.Dialog != nil {
		a.app.Dialog.Error().SetTitle(title).SetMessage(message).Show()
	} else {
		logger.Errorf("[%s] %s", title, message)
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

type Settings struct {
	AutoStart              bool `json:"autoStart"`
	NetworkPrintingEnabled bool `json:"networkPrintingEnabled"`
	DebugMode              bool `json:"debugMode"`
}

type AppVariable struct {
	AppID    string              `json:"appId"`
	Server   ServerInfo          `json:"server"`
	System   SystemInfo          `json:"system"`
	Build    buildinfo.BuildInfo `json:"build"`
	Settings Settings            `json:"settings"`
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

	autoStart := false
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
		Build: buildinfo.Get(),
		Settings: Settings{
			AutoStart:              autoStart,
			NetworkPrintingEnabled: networkEnabled,
			DebugMode:              debugMode,
		},
	}
}

func (a *App) notifyAppVariablesChanged() {
	a.emit("app:variables_changed", a.AppVariable())
}

func (a *App) Printers() printer.DiscoveryResult {
	logger.Debug("Collecting printer status")
	if a.webserver == nil {
		logger.Warn("Webserver is not running, cannot collect printer status")
		return printer.DiscoveryResult{}
	}

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
	if err := a.config.RemoveLANPrinter(ip); err != nil {
		return false, fmt.Errorf("failed to remove LAN printer: %w", err)
	}
	return true, nil
}

func (a *App) CheckLANPrinterStatus(ip string) bool {
	logger.Debugf("Checking LAN printer status: %s", ip)
	return printer.CheckLANPrinter(ip) == nil
}

func (a *App) DownloadLogs() {
	logger.Debugf("Download logs requested")
	logDir := logger.LogDirectory()
	zipName := fmt.Sprintf("obox-app-logs-%s-%s.zip", a.config.GetAppID(), time.Now().Format("2006-01-02"))
	if a.app == nil || a.app.Dialog == nil {
		return
	}
	dlg := a.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Save Archive",
		Filename: zipName,
	})
	dlg.AddFilter("Zip Archives (*.zip)", "*.zip")
	savePath, err := dlg.PromptForSingleSelection()
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
	if a.app.Env != nil {
		_ = a.app.Env.OpenFileManager(savePath, true)
	}
}

// ToggleAutoStart toggles start-at-login using the Wails v3 built-in mechanism.
func (a *App) ToggleAutoStart() error {
	logger.Info("Toggling autostart")
	if a.app != nil && a.app.Autostart != nil {
		enabled, err := a.app.Autostart.IsEnabled()
		if err != nil {
			logger.Errorf("Failed to check autostart status: %v", err)
			return err
		}
		var targetErr error
		if enabled {
			targetErr = a.app.Autostart.Disable()
		} else {
			targetErr = a.app.Autostart.EnableWithOptions(application.AutostartOptions{
				Arguments: []string{"--hidden"},
			})
		}
		if targetErr != nil {
			logger.Errorf("Failed to toggle start at login: %v", targetErr)
			return targetErr
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
	logger.Debugf("Disconnect Odoo requested")
	if a.obox == nil {
		return false, fmt.Errorf("Odoo connection is not initialized")
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
	info, err := a.updater.Check(context.Background())
	if err != nil {
		logger.Errorf("Update check failed: %v", err)
	}
	info.Seen = a.config.LastSeenUpdate() != "" && a.config.LastSeenUpdate() == info.LatestVersion
	if !info.Seen && info.State == updater.StateAvailable {
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

func (a *App) notifyUpdateProgress(p updater.Progress) {
	a.emit("update-progress", p)
}

func (a *App) notifyUpdateStatus(info update.Info) {
	a.emit("update-status", info)
}

func (a *App) DownloadUpdate() (string, error) {
	staged, err := a.updater.Download(context.Background(), func(p updater.Progress) {
		a.notifyUpdateProgress(p)
	})
	a.notifyUpdateStatus(a.updater.Info())
	if err != nil {
		logger.Errorf("Failed to download update: %v", err)
		return "", err
	}

	go func() {
		time.Sleep(5 * time.Second)
		logger.Debugf("Applying update")
		if err := a.ApplyUpdate(); err != nil {
			logger.Errorf("Failed to auto-apply update: %v", err)
			a.notifyUpdateStatus(a.updater.Info())
		}
	}()

	return staged, nil
}

func (a *App) ApplyUpdate() error {
	a.restart.Store(true)

	if err := a.updater.Apply(); err != nil {
		a.restart.Store(false)
		logger.Errorf("Failed to apply update: %v", err)
		a.notifyUpdateStatus(a.updater.Info())
		return err
	}

	a.notifyUpdateStatus(a.updater.Info())

	if update.PlatformNeedsQuitAfterApply() {
		if a.webserver != nil && a.webserver.Running() {
			_ = a.webserver.Stop()
		}
		if a.app != nil {
			a.app.Quit()
		}
	} else {
		a.restart.Store(false)
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
	a.updater.Reset()
	a.notifyUpdateStatus(a.updater.Info())
}
