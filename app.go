package main

import (
	"context"
	"encoding/json"
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
	a.webserver = server.New(port, a.printerManager, a.obox, a.config, assets)
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

// ReloadKiosk broadcasts a kiosk-reload event to reload the active kiosk iframe.
func (a *App) ReloadKiosk() {
	if a.ctx != nil {
		a.events.Emit(a.ctx, "kiosk-reload")
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
		// PIN cancelled or failed: return directly to WebApp in fullscreen
		a.NavigateToWebApp()
	}
}

// NavigateToWebApp navigates the WebView directly to the configured WebApp URL in fullscreen with no menubar.
func (a *App) NavigateToWebApp() {
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
			// Periodically inject non-blocking corner tap & shortcut listener across navigation
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					if a.ctx != nil {
						wailsruntime.WindowExecJS(a.ctx, script)
					}
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

// InjectGestureScript injects the non-blocking exit gesture and hotkey listener into the currently active page.
func (a *App) InjectGestureScript() {
	if a.ctx != nil {
		script := a.getGestureScript()
		wailsruntime.WindowExecJS(a.ctx, script)
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

func (a *App) getGestureScript() string {
	port := 4545
	if a.webserver != nil && a.webserver.Port > 0 {
		port = a.webserver.Port
	}
	wailsAppURL := a.getWailsAppURL()
	exitCorners := a.config.GetWebViewExitCorners()
	cornersJSON, err := json.Marshal(exitCorners)
	if err != nil {
		cornersJSON = []byte(`["top-right"]`)
	}

	return fmt.Sprintf(`(function() {
  if (window.__eposProxyExitInstalled) return;
  window.__eposProxyExitInstalled = true;

  var CORNER_SIZE = 140;
  var REQUIRED_TAPS = 4;
  var RESET_MS = 1000;
  var WAILS_APP_URL = %q;
  var PROXY_PORT = %d;
  var EXIT_CORNERS = %s;

  var tapCount = 0;
  var lastTapTime = 0;
  var activeCorner = null;

  function getTappedCorner(x, y) {
    var w = window.innerWidth || document.documentElement.clientWidth || (document.body ? document.body.clientWidth : 0);
    var h = window.innerHeight || document.documentElement.clientHeight || (document.body ? document.body.clientHeight : 0);
    var corner = null;
    if (x <= CORNER_SIZE && y <= CORNER_SIZE) {
      corner = "top-left";
    } else if (x >= w - CORNER_SIZE && y <= CORNER_SIZE) {
      corner = "top-right";
    } else if (x <= CORNER_SIZE && y >= h - CORNER_SIZE) {
      corner = "bottom-left";
    } else if (x >= w - CORNER_SIZE && y >= h - CORNER_SIZE) {
      corner = "bottom-right";
    }
    if (corner && EXIT_CORNERS && EXIT_CORNERS.indexOf(corner) !== -1) {
      return corner;
    }
    return null;
  }

  function flashIndicator(corner, count) {
    try {
      var dot = document.createElement("div");
      dot.style.position = "fixed";
      dot.style.width = "26px";
      dot.style.height = "26px";
      dot.style.borderRadius = "50%%";
      dot.style.backgroundColor = count >= REQUIRED_TAPS ? "#10B981" : "#EF4444";
      dot.style.zIndex = "2147483647";
      dot.style.pointerEvents = "none";
      dot.style.boxShadow = "0 0 10px rgba(0,0,0,0.5)";
      dot.style.transition = "opacity 0.4s";

      if (corner === "top-left") {
        dot.style.top = "12px"; dot.style.left = "12px";
      } else if (corner === "bottom-left") {
        dot.style.bottom = "12px"; dot.style.left = "12px";
      } else if (corner === "bottom-right") {
        dot.style.bottom = "12px"; dot.style.right = "12px";
      } else {
        // Default: top-right
        dot.style.top = "12px"; dot.style.right = "12px";
      }

      (document.body || document.documentElement).appendChild(dot);
      setTimeout(function() {
        dot.style.opacity = "0";
        setTimeout(function() { dot.remove(); }, 400);
      }, 350);
    } catch (e) {}
  }

  function handleTap(x, y) {
    var corner = getTappedCorner(x, y);
    if (!corner) {
      tapCount = 0;
      activeCorner = null;
      return false;
    }

    var now = Date.now();
    if (activeCorner === corner && (now - lastTapTime) < RESET_MS) {
      tapCount++;
    } else {
      activeCorner = corner;
      tapCount = 1;
    }
    lastTapTime = now;

    flashIndicator(corner, tapCount);

    if (tapCount >= REQUIRED_TAPS) {
      tapCount = 0;
      activeCorner = null;
      triggerExit();
      return true;
    }
    return false;
  }

  function triggerExit() {
    console.log("[ePOS] 4 taps detected on " + (activeCorner || "corner") + ", returning to Wails app");

    // 1. Direct top-level navigation to local proxy exit endpoint
    // Top-level navigation is never blocked by Mixed Content or CORS policies!
    try {
      window.location.replace("http://127.0.0.1:" + PROXY_PORT + "/api/kiosk/exit");
      return;
    } catch(e) {}

    // 2. Direct navigation to WAILS_APP_URL
    try {
      if (WAILS_APP_URL) {
        window.location.replace(WAILS_APP_URL + (WAILS_APP_URL.indexOf("?") >= 0 ? "&" : "?") + "kiosk_exit=1");
        return;
      }
    } catch(e) {}

    // 3. Fallback: beacon fetch
    try {
      fetch("http://127.0.0.1:" + PROXY_PORT + "/api/kiosk/exit", { method: "POST", mode: "no-cors" }).catch(function(){});
    } catch(e) {}
  }

  var lastInputTime = 0;
  function onInput(e) {
    if (e.button && e.button !== 0) return;
    var now = Date.now();
    if (now - lastInputTime < 60) return;
    lastInputTime = now;

    var clientX = e.clientX;
    var clientY = e.clientY;
    if (e.touches && e.touches.length > 0) {
      clientX = e.touches[0].clientX;
      clientY = e.touches[0].clientY;
    }
    if (typeof clientX !== "number" || typeof clientY !== "number") return;

    var triggered = handleTap(clientX, clientY);
    if (triggered) {
      try { e.preventDefault(); e.stopPropagation(); } catch (err) {}
    }
  }

  window.addEventListener("pointerdown", onInput, true);
  window.addEventListener("mousedown", onInput, true);
  window.addEventListener("touchstart", onInput, true);

  window.addEventListener("keydown", function(e) {
    if (e.key === "Escape" || (e.ctrlKey && e.altKey && e.key.toLowerCase() === "s") || e.key === "F11") {
      try { e.preventDefault(); } catch (err) {}
      triggerExit();
    }
  }, true);
})();`, wailsAppURL, port, string(cornersJSON))
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
