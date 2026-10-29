package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

func setupSystemTray(app *application.App, window *application.WebviewWindow, oboxApp *App) {
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetLabel("Obox")
	tray.SetIconPosition(application.NSImageLeft)

	menu := app.NewMenu()

	statusItem := menu.Add("Odoo: disconnected")
	statusItem.SetEnabled(false)
	oboxApp.setTrayStatusItem(statusItem)

	menu.AddSeparator()

	menu.Add("Open").OnClick(func(_ *application.Context) {
		window.Show()
		window.Focus()
	})

	menu.AddSeparator()

	menu.Add("Quit").OnClick(func(_ *application.Context) {
		app.Quit()
	})

	tray.SetMenu(menu)

	showWindow := func() {
		if window.IsVisible() {
			window.Focus()
		} else {
			window.Show()
			window.Focus()
		}
	}

	tray.OnClick(showWindow)
	tray.OnDoubleClick(showWindow)
}
