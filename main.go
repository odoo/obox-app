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
	"context"
	"embed"
	"os"

	"epos-proxy/internal/logger"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logger.InitLogger()
	logger.Debugf("Starting ePOS Proxy")

	app := NewApp()

	windowStartState := options.Normal
	for _, arg := range os.Args[1:] {
		if arg == "--minimized" {
			logger.Debugf("Application started with --minimized flag")
			windowStartState = options.Minimised
			break
		}
	}

	err := wails.Run(&options.App{
		Title:                    "ePOS Proxy",
		Width:                    800,
		Height:                   600,
		MinWidth:                 700,
		MinHeight:                500,
		Menu:                     createMenu(app),
		EnableDefaultContextMenu: true,
		WindowStartState:         windowStartState,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "epos-proxy-single-instance",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				logger.Warn("Second instance detected, focusing existing window")
				wailsruntime.WindowShow(app.ctx)
				wailsruntime.WindowUnminimise(app.ctx)
			},
		},
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if app.ConfirmQuit() {
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
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		logger.Errorf("Application crashed: %v", err)
	}

}
