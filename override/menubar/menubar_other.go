//go:build !linux

package menubar

// SetNativeMenubarVisible is a no-op fallback on Windows and macOS where Wails menu replacement works natively.
func SetNativeMenubarVisible(visible bool) {
}

// DisableContextMenu is a no-op fallback on Windows and macOS.
func DisableContextMenu() {
}
