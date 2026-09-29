// EnableLinuxAutostart creates the XDG autostart entry manually.
//
// We intentionally override the behavior of the go-autostart library on Linux.
// The library generates a `.desktop` file where the Exec path is wrapped in quotes,
// for example:
//
//    Exec="/home/user/app"
//
// Many desktop environments (GNOME/XDG autostart) do not correctly execute quoted
// paths in `.desktop` files, causing the application to fail silently at login.
//
// To avoid this issue we manually create the `.desktop` file with an unquoted
// Exec path:
//
//    Exec=/home/user/app
//
// This ensures reliable autostart behavior across GNOME-based Linux systems.

package util

import (
	"fmt"
	"os"
	"path/filepath"

	"obox-app/internal/logger"
)

func EnableLinuxAutostart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	logger.Debugf("Executable path detected: %s", exe)

	dir := filepath.Join(os.Getenv("HOME"), ".config", "autostart")
	logger.Infof("Ensuring autostart directory exists: %s", dir)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create autostart directory: %w", err)
	}

	file := filepath.Join(dir, "obox-app.desktop")
	logger.Debugf("Creating autostart file: %s", file)

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Obox App
Exec=%s
Terminal=false
StartupNotify=false
X-GNOME-Autostart-enabled=true
`, exe)

	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write autostart file: %w", err)
	}

	logger.Info("Linux autostart successfully enabled")
	return nil
}
