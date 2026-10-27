package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// setupSystemTray creates the native system-tray icon and context menu.
func setupSystemTray(app *application.App, window *application.WebviewWindow, oboxApp *App) {
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetLabel("Obox")
	tray.SetIconPosition(application.NSImageLeft)

	menu := app.NewMenu()

	menu.Add("Open").OnClick(func(_ *application.Context) {
		window.Show()
		window.Focus()
	})

	menu.AddSeparator()

	menu.Add("Quit").OnClick(func(_ *application.Context) {
		go func() {
			if oboxApp.confirmQuit() {
				app.Quit()
			}
		}()
	})

	tray.SetMenu(menu)

	// Click on tray icon: show/focus the window.
	tray.OnClick(func() {
		if window.IsVisible() {
			window.Focus()
		} else {
			window.Show()
			window.Focus()
		}
	})
}
