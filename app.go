package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"obox-app/internal/app_actions"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/obox"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/util"
	"obox-app/override/menubar"

	autostart "github.com/emersion/go-autostart"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/menu"
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

	appMenu           *menu.Menu // stored so kiosk mode can hide/restore the menu bar
	sessionToken      string     // trusted Wails-origin token set once in startup()
	pinAuthMu         sync.RWMutex
	pendingPinAuth    bool
	inManagement      bool
	isRenderingWebApp bool
	wailsAppURL       string
	navStopChan       chan struct{}

	sessionMu         sync.RWMutex
	pendingSession    *pendingOdooSession
	completedSessions map[string]*completedPairing
}

type completedPairing struct {
	Nonce       string
	InstanceID  string
	Token       string
	CompletedAt time.Time
}

type pendingOdooSession struct {
	InstanceID  string
	InstanceURL string
	Nonce       string
	ExpiresAt   time.Time
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

type AppVariable struct {
	AppID                  string `json:"appId"`
	IPAddress              string `json:"ipAddress"`
	ServerRunning          bool   `json:"serverRunning"`
	OS                     string `json:"os"`
	NetworkPrintingEnabled bool   `json:"networkPrintingEnabled"`
	AutoStart              bool   `json:"autoStart"`
	Fullscreen             bool   `json:"fullscreen"`
}

// WebViewConfig is the public view of kiosk settings (PIN is never exposed).
type WebViewConfig struct {
	URL         string   `json:"url"`
	Enabled     bool     `json:"enabled"`
	HasPIN      bool     `json:"hasPIN"`
	ExitCorners []string `json:"exitCorners"`
	Fullscreen  bool     `json:"fullscreen"`
}

func NewApp() *App {
	a := &App{
		dialogs: runtimeDialogs{},
		events:  runtimeEvents{},
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
	}

	a.autoStart = &autostart.App{
		Name:        "obox-app",
		DisplayName: "Obox App",
		Exec:        []string{execPath},
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

	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Debugf("Application startup")
	_, _ = a.startBackend()
}

func (a *App) startBackend() (int, error) {
	logger.Debugf("Config loaded from %s", a.config.Path())

	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	a.printerManager = printer.NewManager(a.config)
	a.obox = obox.NewManager(port, a.config, app_actions.OboxActionHandler(a.printerManager))
	a.obox.SetDeviceProvider(func() []map[string]interface{} {
		return app_actions.BuildDeviceListAsInterface(a.printerManager)
	})
	a.webserver = server.New(port, a.printerManager, a.obox, a.config, assets)
	a.webserver.SetOnCompleteSessionPairing(func(nonce, instanceID, token, dbURL string, oboxID int) error {
		return a.CompleteOdooSessionPairing(nonce, instanceID, token, dbURL, oboxID)
	})
	a.webserver.SetOnOdooSessionExit(func() {
		a.ReturnFromOdooSession()
	})
	a.obox.SetOnStatusChange(func(status obox.ConnectionStatus) {
		a.events.Emit(a.ctx, "odoo:status_changed", status)
	})
	a.obox.SetOnOpenKiosk(func(url string) {
		logger.Infof("Remote command received from Odoo queue: Open WebApp URL %s", url)
		a.NavigateToWebApp()
		if a.ctx != nil {
			a.events.Emit(a.ctx, "kiosk-state-changed", true)
			a.events.Emit(a.ctx, "webview-config-changed")
		}
	})
	a.obox.SetOnReloadKiosk(func() {
		logger.Infof("Remote command received from Odoo queue: Reload Kiosk")
		a.ReloadKiosk()
	})

	// Generate a unique session token that identifies requests from this
	// trusted Wails process. The remote webview never has this token.
	token := uuid.New().String()
	a.sessionToken = token
	a.webserver.SetSessionToken(token)

	// Notify the desktop frontend when kiosk status or config is modified remotely
	a.webserver.SetKioskCallback(func(enabled bool) {
		if enabled {
			logger.Infof("Remote command received: Open WebApp")
			a.NavigateToWebApp()
		} else {
			logger.Infof("Remote command received: Close WebApp")
			a.ReturnToWailsApp()
		}
		if a.ctx != nil {
			a.events.Emit(a.ctx, "kiosk-state-changed", enabled)
		}
	})
	a.webserver.SetConfigCallback(func() {
		if a.ctx != nil {
			a.events.Emit(a.ctx, "webview-config-changed")
		}
	})
	a.webserver.SetKioskReloadCallback(func() {
		if a.ctx != nil {
			a.events.Emit(a.ctx, "kiosk-reload")
		}
	})
	a.webserver.SetKioskExitCallback(func() {
		a.ReturnToWailsApp()
	})
	a.webserver.SetWailsAppURL(a.getWailsAppURL())

	return port, nil
}

func (a *App) shutdown(ctx context.Context) {
	logger.Infof("Stopping Obox App server")

	a.pinAuthMu.Lock()
	if a.navStopChan != nil {
		close(a.navStopChan)
		a.navStopChan = nil
	}
	a.isRenderingWebApp = false
	a.pinAuthMu.Unlock()

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
	networkEnabled := a.config.IsNetworkPrintingEnabled()
	return AppVariable{
		AppID:                  a.config.GetAppID(),
		IPAddress:              util.LocalAddr(a.webserver.Port, networkEnabled),
		OS:                     runtime.GOOS,
		ServerRunning:          a.webserver.Running(),
		AutoStart:              a.autoStart.IsEnabled(),
		NetworkPrintingEnabled: networkEnabled,
		Fullscreen:             a.config.GetWebViewFullscreen(),
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

// ─── WebView / Kiosk ──────────────────────────────────────────────────────────

// GetWebViewConfig returns the public kiosk configuration (URL, enabled flag,
// and whether a PIN has been set). The PIN itself is never returned.
func (a *App) GetWebViewConfig() WebViewConfig {
	return WebViewConfig{
		URL:         a.config.GetWebViewURL(),
		Enabled:     a.config.GetWebViewEnabled(),
		HasPIN:      a.config.HasWebViewPIN(),
		ExitCorners: a.config.GetWebViewExitCorners(),
		Fullscreen:  a.config.GetWebViewFullscreen(),
	}
}

// SetWebViewFullscreen persists the fullscreen kiosk setting and updates window state if active.
func (a *App) SetWebViewFullscreen(fullscreen bool) error {
	logger.Debugf("Setting WebView fullscreen: %v", fullscreen)
	if err := a.config.SetWebViewFullscreen(fullscreen); err != nil {
		return err
	}
	if a.IsRenderingWebApp() {
		a.SetWindowFullscreen(fullscreen)
	}
	if a.ctx != nil {
		a.events.Emit(a.ctx, "webview-config-changed")
	}
	return nil
}

// SetWebViewExitCorners persists the configured corners for the 4-tap exit gesture.
func (a *App) SetWebViewExitCorners(corners []string) error {
	logger.Debugf("Setting WebView exit corners: %v", corners)
	return a.config.SetWebViewExitCorners(corners)
}

// SetWebViewExitCorner persists the configured corner for the 4-tap exit gesture.
func (a *App) SetWebViewExitCorner(corner string) error {
	logger.Debugf("Setting WebView exit corner: %s", corner)
	return a.config.SetWebViewExitCorner(corner)
}

// SetWebViewURL persists the kiosk URL.
func (a *App) SetWebViewURL(url string) error {
	logger.Debugf("Setting WebView URL")
	return a.config.SetWebViewURL(url)
}

// SetWebViewPIN validates and persists the 4-digit kiosk PIN.
func (a *App) SetWebViewPIN(pin string) error {
	logger.Debug("Setting WebView PIN")
	return a.config.SetWebViewPIN(pin)
}

// ValidateWebViewPIN returns true when pin matches the stored PIN.
// The incoming value is compared but never logged.
func (a *App) ValidateWebViewPIN(pin string) bool {
	return a.config.CheckWebViewPIN(pin)
}

// SetWebViewEnabled persists the kiosk-enabled flag.
func (a *App) SetWebViewEnabled(v bool) error {
	logger.Debugf("Setting WebView enabled: %v", v)
	return a.config.SetWebViewEnabled(v)
}

// SetWindowFullscreen puts the main Wails window into or out of fullscreen
// and hides/restores the native menu bar accordingly.
func (a *App) SetWindowFullscreen(fullscreen bool) {
	if a.ctx == nil {
		return
	}
	if fullscreen {
		wailsruntime.WindowFullscreen(a.ctx)
		menubar.SetNativeMenubarVisible(false)
		if runtime.GOOS != "linux" {
			wailsruntime.MenuSetApplicationMenu(a.ctx, menu.NewMenu())
			wailsruntime.MenuUpdateApplicationMenu(a.ctx)
		}
	} else {
		wailsruntime.WindowUnfullscreen(a.ctx)
		menubar.SetNativeMenubarVisible(true)
		if runtime.GOOS != "linux" {
			if a.appMenu == nil {
				a.appMenu = createMenu(a)
			}
			wailsruntime.MenuSetApplicationMenu(a.ctx, a.appMenu)
			wailsruntime.MenuUpdateApplicationMenu(a.ctx)
		}
	}
}

// ReloadKiosk broadcasts a kiosk-reload event or reloads the active webview window.
func (a *App) ReloadKiosk() {
	if a.ctx != nil {
		if a.IsRenderingWebApp() {
			wailsruntime.WindowExecJS(a.ctx, "window.location.reload();")
		} else {
			a.events.Emit(a.ctx, "kiosk-reload")
		}
	}
}

// IsPendingPinAuth returns true if the Wails UI was opened from WebApp and requires PIN auth.
func (a *App) IsPendingPinAuth() bool {
	a.pinAuthMu.RLock()
	defer a.pinAuthMu.RUnlock()
	return a.pendingPinAuth
}

// IsInManagement returns true if the user is authenticated and currently viewing the Wails management UI.
func (a *App) IsInManagement() bool {
	a.pinAuthMu.RLock()
	defer a.pinAuthMu.RUnlock()
	return a.inManagement
}

// IsRenderingWebApp returns true if the top-level WebView is currently navigated to the external WebApp.
func (a *App) IsRenderingWebApp() bool {
	a.pinAuthMu.RLock()
	defer a.pinAuthMu.RUnlock()
	return a.isRenderingWebApp
}

// SetWailsAppURL records the local Wails UI URL so we can return to it cleanly.
func (a *App) SetWailsAppURL(url string) {
	a.pinAuthMu.Lock()
	defer a.pinAuthMu.Unlock()
	if url != "" {
		a.wailsAppURL = url
		if a.webserver != nil {
			a.webserver.SetWailsAppURL(url)
		}
		logger.Infof("Recorded Wails App URL: %s", url)
	}
}

// CompletePinAuth is called by the Wails frontend when PIN validation succeeds or is cancelled/failed.
func (a *App) CompletePinAuth(success bool) {
	logger.Infof("CompletePinAuth: success=%v", success)
	if success {
		a.pinAuthMu.Lock()
		a.pendingPinAuth = false
		a.inManagement = true
		a.isRenderingWebApp = false
		a.pinAuthMu.Unlock()
		a.SetWindowFullscreen(false)
	} else {
		// PIN cancelled or failed: return to WebApp only if enabled, else return to Wails app
		if a.config.GetWebViewEnabled() {
			a.NavigateToWebApp()
		} else {
			a.ReturnToWailsApp()
		}
	}
}

// NavigateToWebApp navigates the WebView directly to the configured WebApp URL in fullscreen with no menubar.
func (a *App) NavigateToWebApp() {
	if !a.config.GetWebViewEnabled() {
		logger.Infof("NavigateToWebApp skipped: webview_enabled is false")
		return
	}

	a.pinAuthMu.Lock()
	a.pendingPinAuth = false
	a.inManagement = false
	a.isRenderingWebApp = true
	if a.navStopChan != nil {
		close(a.navStopChan)
	}
	stopCh := make(chan struct{})
	a.navStopChan = stopCh
	a.pinAuthMu.Unlock()

	targetURL := a.config.GetWebViewURL()
	if targetURL == "" {
		return
	}

	if a.config.GetWebViewFullscreen() {
		a.SetWindowFullscreen(true)
	} else {
		a.SetWindowFullscreen(false)
	}

	if a.ctx != nil {
		wailsruntime.WindowShow(a.ctx)
		wailsruntime.WindowUnminimise(a.ctx)
		logger.Infof("Navigating top-level WebView to configured URL: %s", targetURL)
		wailsruntime.WindowExecJS(a.ctx, fmt.Sprintf("window.location.replace(%q);", targetURL))

		script := a.getGestureScript()
		go func() {
			select {
			case <-stopCh:
				return
			case <-time.After(1500 * time.Millisecond):
				a.pinAuthMu.Lock()
				isRendering := a.isRenderingWebApp
				a.pinAuthMu.Unlock()
				if isRendering && a.ctx != nil {
					wailsruntime.WindowExecJS(a.ctx, script)
				}
			}
		}()
	}
}

// ReturnToWailsApp returns from WebApp to the Wails app and requires PIN authentication before granting access.
func (a *App) ReturnToWailsApp() {
	logger.Infof("Returning to Wails app from WebApp")
	a.pinAuthMu.Lock()
	a.pendingPinAuth = true
	a.inManagement = false
	a.isRenderingWebApp = false
	if a.navStopChan != nil {
		close(a.navStopChan)
		a.navStopChan = nil
	}
	target := a.wailsAppURL
	a.pinAuthMu.Unlock()

	if target == "" {
		target = a.getWailsAppURL()
	}

	a.SetWindowFullscreen(false)

	if a.ctx != nil {
		logger.Infof("Navigating WebView back to Wails app URL: %s", target)
		wailsruntime.WindowExecJS(a.ctx, fmt.Sprintf("window.location.replace(%q);", target))
	}
}

// ReturnFromOdooSession returns from Odoo session pairing back to the Wails management app without requiring PIN.
func (a *App) ReturnFromOdooSession() {
	logger.Infof("Returning to Wails app from Odoo session")
	a.pinAuthMu.Lock()
	a.pendingPinAuth = false
	a.inManagement = true
	a.isRenderingWebApp = false
	if a.navStopChan != nil {
		close(a.navStopChan)
		a.navStopChan = nil
	}
	target := a.wailsAppURL
	a.pinAuthMu.Unlock()

	if target == "" {
		target = a.getWailsAppURL()
	}

	a.SetWindowFullscreen(false)

	if a.ctx != nil {
		logger.Infof("Navigating WebView back to Wails app URL: %s", target)
		wailsruntime.WindowExecJS(a.ctx, fmt.Sprintf("window.location.replace(%q);", target))
	}
}

// InjectGestureScript injects the non-blocking exit gesture and hotkey listener into the currently active page.
func (a *App) InjectGestureScript() {
	if a.ctx != nil {
		script := a.getGestureScript()
		wailsruntime.WindowExecJS(a.ctx, script)

		a.sessionMu.Lock()
		pending := a.pendingSession
		a.sessionMu.Unlock()
		if pending != nil {
			var target *config.OdooInstance
			for _, inst := range a.config.GetOdooInstances() {
				if inst.ID == pending.InstanceID {
					instCopy := inst
					target = &instCopy
					break
				}
			}
			if target != nil {
				sessionScript := a.getOdooSessionScript(target, pending.Nonce)
				wailsruntime.WindowExecJS(a.ctx, sessionScript)
			}
		}
	}
}

func (a *App) getWailsAppURL() string {
	a.pinAuthMu.RLock()
	url := a.wailsAppURL
	a.pinAuthMu.RUnlock()
	if url != "" {
		return url
	}
	// In Wails dev mode, the app is served via the Vite dev server at 127.0.0.1:5173
	client := http.Client{Timeout: 200 * time.Millisecond}
	if resp, err := client.Get("http://127.0.0.1:5173/"); err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return "http://127.0.0.1:5173/"
		}
	}
	if runtime.GOOS == "windows" {
		return "https://wails.localhost/"
	}
	return "wails://app/index.html"
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

func (a *App) confirmQuit() bool {
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

func (a *App) Quit() {
	logger.Infof("Quit requested via App.Quit")
	if a.webserver != nil {
		_ = a.webserver.Stop()
	}
	os.Exit(0)
}

// GetSessionToken returns the per-launch session token that identifies HTTP
// requests from this trusted Wails process. Called once by the frontend on
// startup; the token is never embedded in the built JS bundle.
func (a *App) GetSessionToken() string {
	return a.sessionToken
}

// GetOdooInstances returns all configured Odoo instances.
func (a *App) GetOdooInstances() []config.OdooInstance {
	return a.config.GetOdooInstances()
}

// AddOdooInstance adds and normalizes a new Odoo instance.
func (a *App) AddOdooInstance(rawURL, name string) (config.OdooInstance, error) {
	inst, err := a.config.AddOdooInstance(rawURL, name)
	if err != nil {
		return config.OdooInstance{}, err
	}
	if a.ctx != nil {
		a.events.Emit(a.ctx, "odoo-instances-changed", a.config.GetOdooInstances())
	}
	return inst, nil
}

// RemoveOdooInstance deletes an Odoo instance by ID.
func (a *App) RemoveOdooInstance(id string) error {
	err := a.config.RemoveOdooInstance(id)
	if err != nil {
		return err
	}
	if !a.config.HasOdooCredentials() && a.obox != nil {
		a.obox.Disconnect()
	}
	if a.ctx != nil {
		a.events.Emit(a.ctx, "odoo-instances-changed", a.config.GetOdooInstances())
		if a.obox != nil {
			a.events.Emit(a.ctx, "odoo:status_changed", a.obox.ConnectionStatus())
		}
	}
	return nil
}

// SetActiveOdooInstance switches active pairing to an already paired instance.
func (a *App) SetActiveOdooInstance(id string) error {
	err := a.config.SetActiveOdooInstance(id)
	if err != nil {
		return err
	}
	active := a.config.GetActiveOdooInstance()
	if active != nil && active.Token != "" && a.obox != nil {
		a.obox.Connect(active.URL, active.Token)
	}
	if a.ctx != nil {
		a.events.Emit(a.ctx, "odoo-instances-changed", a.config.GetOdooInstances())
		if a.obox != nil {
			a.events.Emit(a.ctx, "odoo:status_changed", a.obox.ConnectionStatus())
		}
	}
	return nil
}

// OpenOdooSession navigates the top-level WebView to the chosen Odoo instance with pairing support.
func (a *App) OpenOdooSession(id string) error {
	var target *config.OdooInstance
	for _, inst := range a.config.GetOdooInstances() {
		if inst.ID == id {
			instCopy := inst
			target = &instCopy
			break
		}
	}
	if target == nil {
		return errors.New("odoo instance not found")
	}

	nonce := uuid.New().String()
	a.sessionMu.Lock()
	a.pendingSession = &pendingOdooSession{
		InstanceID:  id,
		InstanceURL: target.URL,
		Nonce:       nonce,
		ExpiresAt:   time.Now().Add(30 * time.Minute),
	}
	a.sessionMu.Unlock()

	a.navigateToOdooSession(target, nonce)
	return nil
}

func (a *App) navigateToOdooSession(target *config.OdooInstance, nonce string) {
	a.pinAuthMu.Lock()
	a.pendingPinAuth = false
	a.inManagement = false
	a.isRenderingWebApp = true
	if a.navStopChan != nil {
		close(a.navStopChan)
	}
	stopCh := make(chan struct{})
	a.navStopChan = stopCh
	a.pinAuthMu.Unlock()

	targetURL := target.URL
	if a.config.GetWebViewFullscreen() {
		a.SetWindowFullscreen(true)
	} else {
		a.SetWindowFullscreen(false)
	}

	if a.ctx != nil {
		wailsruntime.WindowShow(a.ctx)
		wailsruntime.WindowUnminimise(a.ctx)
		logger.Infof("Navigating top-level WebView to Odoo instance: %s", targetURL)
		wailsruntime.WindowExecJS(a.ctx, fmt.Sprintf("window.location.replace(%q);", targetURL))

		script := a.getOdooSessionScript(target, nonce)
		gestureScript := a.getGestureScript()
		go func() {
			for _, d := range []time.Duration{800 * time.Millisecond, 2000 * time.Millisecond, 4000 * time.Millisecond} {
				select {
				case <-stopCh:
					return
				case <-time.After(d):
					a.pinAuthMu.Lock()
					isRendering := a.isRenderingWebApp
					a.pinAuthMu.Unlock()
					if isRendering && a.ctx != nil {
						wailsruntime.WindowExecJS(a.ctx, script)
						wailsruntime.WindowExecJS(a.ctx, gestureScript)
					}
				}
			}
		}()
	}
}

func (a *App) getOdooSessionScript(target *config.OdooInstance, nonce string) string {
	if target != nil && target.Token != "" {
		return ""
	}

	platform := "android" //runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}

	return fmt.Sprintf(`(function() {
  function mount() {
    var instanceID = %q;
    var nonce = %q;
    var serial = %q;
    var platform = %q;
    var wailsURL = %q;
    var proxyPort = %d;

    if (sessionStorage.getItem("__obox_paired_" + instanceID) === "1") return;
    if (document.getElementById("__obox_session_bar")) return;
    if (!document.body && !document.documentElement) return;

    var bar = document.createElement("div");
    bar.id = "__obox_session_bar";
    bar.innerHTML = '<div style="position:fixed;top:12px;right:20px;z-index:2147483647;display:flex;align-items:center;gap:8px;background:rgba(17,24,39,0.95);color:#fff;padding:6px 14px;border-radius:9999px;font-size:12px;font-family:sans-serif;box-shadow:0 4px 16px rgba(0,0,0,0.4);border:1px solid rgba(255,255,255,0.2);user-select:none;">' +
      '<span style="font-weight:700;color:#A78BFA;">OBOX</span><span>•</span>' +
      '<button id="__obox_pair_btn" style="background:#714B67;color:#fff;border:none;border-radius:9999px;padding:4px 12px;font-weight:600;font-size:11px;cursor:pointer;">Pair with this Odoo</button>' +
      '<button id="__obox_exit_btn" style="background:transparent;color:#9CA3AF;border:none;font-size:11px;cursor:pointer;padding:2px 6px;">Exit</button>' +
      '</div>';

    var btn = bar.querySelector("#__obox_pair_btn");
    var exitBtn = bar.querySelector("#__obox_exit_btn");

    exitBtn.onclick = function() {
      try {
        fetch("http://127.0.0.1:" + proxyPort + "/api/odoo/exit_session", { method: "POST" }).catch(function(){});
      } catch (e) {}
      try {
        window.location.replace(wailsURL);
      } catch (e) {
        window.location.href = wailsURL;
      }
    };

    btn.onclick = async function() {
      btn.disabled = true;
      btn.innerText = "Pairing...";

      var name = "OBOX App";

      try {
        var res = null;
        var orm = window.odoo?.__WOWL_DEBUG__?.root?.env?.services?.orm;
        if (orm && typeof orm.call === "function") {
          res = await orm.call("obox.obox", "pair_mobile_obox", [serial], {
            platform: platform,
            name: name
          });
        } else {
          var r = await fetch("/web/dataset/call_kw/obox.obox/pair_mobile_obox", {
            method: "POST",
            credentials: "include",
            headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
            body: JSON.stringify({
              jsonrpc: "2.0",
              method: "call",
              id: Date.now(),
              params: {
                model: "obox.obox",
                method: "pair_mobile_obox",
                args: [serial],
                kwargs: { platform: platform, name: name }
              }
            })
          });
          var data = await r.json();
          if (data.error) {
            throw new Error(data.error.data?.message || data.error.message || JSON.stringify(data.error));
          }
          res = data.result;
        }

        if (!res || !res.token) {
          throw new Error("Unexpected pairing response from Odoo: " + JSON.stringify(res));
        }

        btn.innerText = "✓ Paired!";
        btn.style.background = "#059669";
        btn.disabled = true;
        sessionStorage.setItem("__obox_paired_" + instanceID, "1");

        var payload = {
          nonce: nonce,
          instanceID: instanceID,
          token: res.token,
          oboxID: res.id || 0,
          dbURL: res.url || window.location.origin
        };

        try {
          await fetch("http://127.0.0.1:" + proxyPort + "/api/odoo/complete_session_pairing", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload)
          });
        } catch (localErr) {
          console.warn("Could not notify local server of pairing:", localErr);
        }
      } catch (err) {
        if (res && res.token) {
          btn.innerText = "✓ Paired!";
          btn.style.background = "#059669";
          btn.disabled = true;
          return;
        }
        alert("OBOX Pairing Error: " + err.message + "\n\nPlease ensure you are logged into Odoo on this page.");
        btn.disabled = false;
        btn.innerText = "Pair with this Odoo";
      }
    };

    (document.body || document.documentElement).appendChild(bar);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount);
  } else {
    mount();
  }
  setInterval(mount, 2000);
})();`,
		target.ID,
		nonce,
		a.config.GetAppID(),
		platform,
		a.getWailsAppURL(),
		a.config.GetPort(),
	)
}

// CompleteOdooSessionPairing completes the pairing from the Wails app frontend after returning from Odoo.
func (a *App) CompleteOdooSessionPairing(nonce, instanceID, token, dbURL string, oboxID int) error {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()

	// 1. If this pairing was already completed recently with the same nonce, succeed idempotently.
	if a.completedSessions != nil {
		if comp, ok := a.completedSessions[nonce]; ok && comp.InstanceID == instanceID && time.Since(comp.CompletedAt) < 15*time.Minute {
			logger.Infof("Session pairing for nonce %s was already completed successfully", nonce)
			return nil
		}
	}

	// 2. If instance is already paired with this token, succeed idempotently.
	inst := a.config.GetOdooInstance(instanceID)
	if inst != nil && inst.Token != "" && inst.Token == token {
		logger.Infof("Instance %s is already paired with token", instanceID)
		return nil
	}

	pending := a.pendingSession
	if pending == nil || pending.Nonce != nonce || pending.InstanceID != instanceID || time.Now().After(pending.ExpiresAt) {
		return errors.New("invalid or expired session pairing nonce")
	}

	if err := a.config.SaveOdooInstancePairing(instanceID, token, oboxID); err != nil {
		return err
	}

	if dbURL == "" {
		dbURL = pending.InstanceURL
	}
	if a.obox != nil {
		a.obox.Connect(dbURL, token)
		go func() {
			time.Sleep(300 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := a.obox.Sync(ctx); err != nil {
				logger.Warnf("[obox] Immediate sync after pairing error: %v", err)
			}
		}()
	}

	if a.completedSessions == nil {
		a.completedSessions = make(map[string]*completedPairing)
	}
	a.completedSessions[nonce] = &completedPairing{
		Nonce:       nonce,
		InstanceID:  instanceID,
		Token:       token,
		CompletedAt: time.Now(),
	}
	a.pendingSession = nil

	if a.ctx != nil {
		a.events.Emit(a.ctx, "odoo-instances-changed", a.config.GetOdooInstances())
		if a.obox != nil {
			a.events.Emit(a.ctx, "odoo:status_changed", a.obox.ConnectionStatus())
		}
	}
	return nil
}

// SyncOdooInstance manually triggers a sync of device state and devices with the given Odoo instance.
func (a *App) SyncOdooInstance(id string) error {
	if id == "" {
		active := a.config.GetActiveOdooInstance()
		if active == nil {
			return errors.New("no active Odoo instance")
		}
		id = active.ID
	}

	inst := a.config.GetOdooInstance(id)
	if inst == nil {
		return errors.New("odoo instance not found")
	}
	if inst.Token == "" {
		return errors.New("odoo instance is not paired")
	}

	if a.obox == nil {
		return errors.New("obox manager not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	a.obox.Connect(inst.URL, inst.Token)

	if err := a.obox.Sync(ctx); err != nil {
		logger.Warnf("[obox] Manual sync for instance %s failed: %v", id, err)
		return fmt.Errorf("sync failed: %w", err)
	}

	logger.Infof("[obox] Manual sync for instance %s succeeded", id)
	return nil
}
