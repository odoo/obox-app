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

	oboxApp.setApp(app)

	startHidden := false
	for _, arg := range os.Args[1:] {
		if arg == "--hidden" || arg == "--minimized" {
			startHidden = true
			break
		}
	}

	// ── Main window ────────────────────────────────────────────────────────
	// Opens the main window by default. Only starts hidden in the system tray
	// if launched with --hidden or --minimized (e.g. system autostart).
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     " ",
		Width:     800,
		Height:    600,
		MinWidth:  700,
		MinHeight: 500,
		Hidden:    startHidden,
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

	setupSystemTray(app, window, oboxApp)
	if err := app.Run(); err != nil {
		logger.Errorf("Application crashed: %v", err)
	}
}
