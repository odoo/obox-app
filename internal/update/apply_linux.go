//go:build linux

package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"obox-app/internal/logger"
)

func Apply(stagedPath string) error {
	logger.Debugf("Applying update to Linux executable from staged path: %s", stagedPath)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("apply failed: cannot locate executable: %w", err)
	}

	// Resolve symlinks to target the real executable.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		exe = resolved
	}
	logger.Debugf("Applying update: executable=%s, staged=%s", exe, stagedPath)

	return applyToExecutable(exe, stagedPath)
}

// applyToExecutable stages the update in the executable's directory and
// atomically replaces the existing executable. The destination directory must
// be writable by the current user. Privileged locations such as /usr/local/bin
// require the user or an external installer; we deliberately avoid a custom
// privileged helper to keep this updater small and auditable.
//
// The function does not restart the application. The caller is responsible for
// notifying the user to restart so the new binary takes effect.
func applyToExecutable(exePath, stagedPath string) error {
	if stagedPath == "" {
		return fmt.Errorf("%w: empty staged path", ErrUpdateNotReady)
	}

	stagedInfo, err := os.Stat(stagedPath)
	if err != nil {
		// Use %v for the inner OS error — only one %w sentinel per Errorf.
		return fmt.Errorf("%w: cannot access staged update %s: %v", ErrUpdateNotReady, stagedPath, err)
	}
	if !stagedInfo.Mode().IsRegular() || stagedInfo.Size() == 0 {
		return fmt.Errorf("%w: staged update must be a non-empty regular file", ErrUpdateNotReady)
	}

	exeDir := filepath.Dir(exePath)

	// Create the temporary file in the destination directory so the final
	// rename stays on the same filesystem (avoids EXDEV cross-device errors).
	// If CreateTemp fails the directory is not writable; return a clear error
	// and let the user or an installer handle privileged locations.
	tmp, err := os.CreateTemp(exeDir, ".obox-update-*")
	if err != nil {
		return fmt.Errorf(
			"apply failed: cannot stage update in %s; check directory permissions or update manually: %w",
			exeDir, err,
		)
	}

	tmpPath := tmp.Name()
	// renamed is set to true once os.Rename succeeds so the defer only
	// removes the temp file on failure paths. After rename, tmpPath no longer
	// exists, but being explicit avoids a redundant syscall and makes the
	// cleanup intent clear.
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpPath)
		}
	}()

	logger.Debugf("Staging update: %s -> %s", stagedPath, tmpPath)

	src, err := os.Open(stagedPath)
	if err != nil {
		_ = tmp.Close()
		return fmt.Errorf("apply failed: cannot open staged update: %w", err)
	}

	_, copyErr := io.Copy(tmp, src)
	srcCloseErr := src.Close()
	if copyErr != nil {
		_ = tmp.Close()
		return fmt.Errorf("apply failed: cannot copy staged update: %w", copyErr)
	}
	if srcCloseErr != nil {
		_ = tmp.Close()
		return fmt.Errorf("apply failed: cannot close staged update source: %w", srcCloseErr)
	}

	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("apply failed: cannot set staged executable permissions: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("apply failed: cannot sync staged executable: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("apply failed: cannot close staged executable: %w", err)
	}

	// On Linux, rename atomically replaces an existing destination on the
	// same filesystem. The running process continues using its old inode.
	// If rename fails, the existing executable remains untouched.
	logger.Debugf("Atomically replacing executable: %s -> %s", tmpPath, exePath)
	if err := os.Rename(tmpPath, exePath); err != nil {
		return fmt.Errorf(
			"apply failed: cannot replace executable at %s; check directory permissions: %w",
			exePath, err,
		)
	}
	renamed = true

	// Remove the staged download file now that it has been consumed.
	if err := os.Remove(stagedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warnf("Could not remove staged update file %s: %v", stagedPath, err)
	}

	logger.Infof("Successfully updated executable at %s — please restart the application", exePath)

	return nil
}

// PlatformNeedsQuitAfterApply reports whether the application must quit
// immediately after Apply() returns so the update can take effect.
//
// On Linux the binary is replaced atomically in-place (the running process
// keeps its old inode), so quitting is not required — we can just tell the
// user to restart at their convenience.
func PlatformNeedsQuitAfterApply() bool { return false }
