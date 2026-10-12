//go:build !windows

package main

func isWindowsService() bool {
	return false
}

func runWindowsService(app *App) {
	// No-op on non-Windows platforms
}
