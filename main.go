package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"slices"

	"obox-app/internal/logger"
	"obox-app/override/menubar"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Permissions-Policy", "local-network-access=*, private-network-access=*, local-network=*, loopback-network=*")
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Configure WebView2 browser arguments to permit Local Network Access (LNA / PNA)
	// and bypass restrictive preflight blocking inside embedded webviews where user prompts cannot be shown.
	pnaFlags := "--disable-features=PrivateNetworkAccessSendPreflights,PrivateNetworkAccessRespectPreflightResults,BlockInsecurePrivateNetworkRequests"
	if existing := os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"); existing == "" {
		_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", pnaFlags)
	} else {
		_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", existing+" "+pnaFlags)
	}

	logger.InitLogger()
	logger.Debugf("Starting Obox App")

	app := NewApp()

	if isWindowsService() {
		logger.Infof("Detected Windows Service context. Starting in native service mode.")
		runWindowsService(app)
		return
	}

	forceKiosk := false
	for _, arg := range os.Args[1:] {
		if arg == "--kiosk" || arg == "-kiosk" || arg == "--server" || arg == "-server" {
			forceKiosk = true
			break
		}
	}

	mode := determineLaunchMode(false, forceKiosk, false)
	if mode == ModeServer {
		runServerMode(app)
		return
	}

	runNormalMode(app)
}

func runNormalMode(app *App) {
	logger.Infof("Starting application in normal Wails mode")

	windowStartState := options.Normal
	if slices.Contains(os.Args[1:], "--minimized") {
		logger.Debugf("Application started with --minimized flag")
		windowStartState = options.Minimised
	}

	appMenu := createMenu(app)
	app.appMenu = appMenu

	err := wails.Run(&options.App{
		Title:                    "",
		Width:                    800,
		Height:                   600,
		MinWidth:                 700,
		MinHeight:                500,
		Menu:                     appMenu,
		EnableDefaultContextMenu: false,
		WindowStartState:         windowStartState,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: securityHeadersMiddleware,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "obox-app-single-instance",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				logger.Warn("Second instance detected, focusing existing window")
				wailsruntime.WindowShow(app.ctx)
				wailsruntime.WindowUnminimise(app.ctx)
			},
		},
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if app.confirmQuit() {
				logger.Infof("User confirmed quit")
				return false
			}

			logger.Infof("Close requested, minimizing window instead of quitting")
			wailsruntime.WindowMinimise(ctx)
			return true
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnDomReady: func(ctx context.Context) {
			menubar.DisableContextMenu()
			if !app.IsRenderingWebApp() {
				if !app.IsPendingPinAuth() && !app.IsInManagement() {
					if app.config.GetWebViewEnabled() {
						if targetURL := app.config.GetWebViewURL(); targetURL != "" {
							app.NavigateToWebApp()
							return
						}
					}
				}
			} else {
				app.InjectGestureScript()
			}
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			DisableWindowIcon: true,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		Linux: &linux.Options{},
	})

	if err != nil {
		logger.Errorf("Application crashed: %v", err)
	}

	if app.webserver != nil {
		_ = app.webserver.Stop()
	}
	os.Exit(0)
}
