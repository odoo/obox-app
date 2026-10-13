//go:build windows

package update

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows"

	"obox-app/internal/logger"
)

var (
	windowsApplyMu   sync.Mutex
	installerStarted bool
)

// Apply launches the downloaded installer with elevation and normal window
// visibility. The caller must quit the running application so that its
// executable file lock is released before the installer attempts to overwrite it.
func Apply(downloaded string) error {
	windowsApplyMu.Lock()
	defer windowsApplyMu.Unlock()

	if installerStarted {
		return fmt.Errorf("%w: installer already started", ErrUpdateInProgress)
	}

	if downloaded == "" {
		return fmt.Errorf("%w: empty installer path", ErrUpdateNotReady)
	}

	info, err := os.Stat(downloaded)
	if err != nil {
		err = fmt.Errorf("%w: downloaded file not found at %s: %v", ErrUpdateNotReady, downloaded, err)
		logger.Errorf("%v", err)
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("%w: installer file is empty", ErrUpdateNotReady)
	}

	logger.Infof("Launching Windows installer: %s", downloaded)

	verb := windows.StringToUTF16Ptr("open")
	file := windows.StringToUTF16Ptr(downloaded)
	dir := windows.StringToUTF16Ptr(filepath.Dir(downloaded))

	if err := windows.ShellExecute(0, verb, file, nil, dir, windows.SW_SHOWNORMAL); err != nil {
		err = fmt.Errorf("apply failed: cannot launch installer %s: %w", downloaded, err)
		logger.Errorf("%v", err)
		return err
	}

	installerStarted = true
	logger.Infof("Installer launched successfully: %s", downloaded)
	return nil
}
