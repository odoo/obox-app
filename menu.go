package main

import (
	"runtime"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func createMenu(app *App) *menu.Menu {
	mainMenu := menu.NewMenu()
	appMenu := mainMenu.AddSubmenu("App")

	if runtime.GOOS == "darwin" {
		// Without an Edit menu, copy/paste do nothing on the webview on macO
		mainMenu.Append(menu.EditMenu())
	}

	appMenu.AddText("Open WebApp", nil, func(_ *menu.CallbackData) {
		app.NavigateToWebApp()
	})

	appMenu.AddText("Return to Wails Management", keys.Combo("s", keys.CmdOrCtrlKey, keys.OptionOrAltKey), func(_ *menu.CallbackData) {
		app.ReturnToWailsApp()
	})

	appMenu.AddCheckbox("Fullscreen WebApp", app.config.GetWebViewFullscreen(), nil, func(cb *menu.CallbackData) {
		_ = app.SetWebViewFullscreen(cb.MenuItem.Checked)
	})

	appMenu.AddText("Set PIN", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "open-set-pin-dialog")
	})

	return mainMenu
}
