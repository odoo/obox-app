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

// Apply opens the downloaded NSIS installer via ShellExecute. UAC elevation is
// handled by the installer's own manifest (RequestExecutionLevel admin), not by
// this call. The caller must quit the running application so that its
// executable file lock is released before the installer attempts to overwrite it.
func applyStaged(downloaded string) error {
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
	logger.Debugf("Applying Windows update: installerPath=%s (size=%d bytes)", downloaded, info.Size())

	verb := windows.StringToUTF16Ptr("open")
	file := windows.StringToUTF16Ptr(downloaded)
	dir := windows.StringToUTF16Ptr(filepath.Dir(downloaded))

	logger.Debugf("Calling windows.ShellExecute verb 'open' on installer: %s", downloaded)
	if err := windows.ShellExecute(0, verb, file, nil, dir, windows.SW_SHOWNORMAL); err != nil {
		// If "open" fails (e.g. permission/access issue), retry with "runas" to explicitly trigger UAC prompt
		logger.Debugf("ShellExecute 'open' failed (%v), retrying with verb 'runas' for UAC elevation", err)
		runasVerb := windows.StringToUTF16Ptr("runas")
		if runasErr := windows.ShellExecute(0, runasVerb, file, nil, dir, windows.SW_SHOWNORMAL); runasErr == nil {
			installerStarted = true
			logger.Infof("Installer launched successfully with runas: %s", downloaded)
			return nil
		}
		err = fmt.Errorf("apply failed: cannot launch installer %s: %w", downloaded, err)
		logger.Errorf("%v", err)
		return err
	}

	installerStarted = true
	logger.Infof("Installer launched successfully: %s", downloaded)
	return nil
}

// PlatformNeedsQuitAfterApply reports whether the application must quit
// immediately after Apply() returns so the update can take effect.
//
// On Windows the NSIS installer needs the running process to exit so it can
// overwrite the locked executable file.
func PlatformNeedsQuitAfterApply() bool { return true }
