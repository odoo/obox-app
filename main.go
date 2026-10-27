// NOTE: this block is currently INERT. cgo only honours a preamble that
// immediately precedes the `import "C"` line; because "C" is imported inside
// the grouped import block below, these flags have never been applied and
// libusb is linked dynamically via gousb's pkg-config. Left as-is pending a
// decision — activating it needs the include path corrected to
// -I/opt/homebrew/opt/libusb/include for <libusb-1.0/libusb.h> to resolve.
/*
#cgo darwin CFLAGS:  -I/opt/homebrew/opt/libusb/include/libusb-1.0
#cgo darwin LDFLAGS: /opt/homebrew/opt/libusb/lib/libusb-1.0.a -framework IOKit -framework CoreFoundation
#include <libusb-1.0/libusb.h>
*/
package main

import (
	"C"
	"embed"
	"os"
	"runtime"

	"obox-app/internal/logger"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	if runtime.GOOS == "linux" && os.Getenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
	}
}

func main() {
	logger.InitLogger()
	logger.Debugf("Starting Obox App")

	oboxApp := NewApp()
	notificationService := notifications.New()
	oboxApp.setNotifications(notificationService)

	var app *application.App

	app = application.New(application.Options{
		Name:        "Obox App",
		Description: "POS Printer Proxy for Odoo",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(oboxApp),
			application.NewService(notificationService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// Keep the app alive in the dock/menu bar when the window is closed.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		// Single-instance lock: if a second instance is launched, focus the first.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "obox-app-single-instance",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				logger.Warn("Second instance detected, focusing existing window")
				if app != nil {
					if win, ok := app.Window.GetByName("main"); ok {
						win.Show()
						win.Focus()
					}
				}
			},
		},
	})

	// Give the service a reference to the app so it can emit events, show
	// dialogs, and manipulate windows.
	oboxApp.setApp(app)

	// ── Main window ────────────────────────────────────────────────────────
	// Application starts in the system tray only. The window stays hidden in the
	// background until the user clicks the tray icon or chooses 'Show Obox App'.
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "Obox App",
		Width:     800,
		Height:    600,
		MinWidth:  700,
		MinHeight: 500,
		Hidden:    true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})

	// Intercept window close: stay running in background as system tray (hidden, not minimized)
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if oboxApp.restart.Load() {
			return
		}
		e.Cancel()
		window.Hide()
	})

	oboxApp.setWindow(window)

	// ── Startup hook ───────────────────────────────────────────────────────
	// ApplicationStarted fires once the event loop is running and all
	// services/windows are initialised — safe to start background services.
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		oboxApp.startup()
	})

	app.OnShutdown(func() {
		oboxApp.shutdown()
	})

	// ── System tray ────────────────────────────────────────────────────────
	setupSystemTray(app, window, oboxApp)

	// ── Run ────────────────────────────────────────────────────────────────
	if err := app.Run(); err != nil {
		logger.Errorf("Application crashed: %v", err)
	}
}
