//go:build !windows

package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"obox-app/internal/logger"
)

// Apply stages the verified binary over the running executable and restarts it.
func Apply(stagedPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("apply failed: cannot locate executable: %w", err)
	}

	// Resolve symlinks to target the real executable
	resolvedExe, err := filepath.EvalSymlinks(exe)
	if err == nil && resolvedExe != "" {
		exe = resolvedExe
	}

	return applyToExecutable(os.Getpid(), exe, stagedPath)
}

// applyToExecutable performs atomic in-place replacement of the executable
// directly in Go with automatic backup, rollback on failure, and detached relaunch.
// On Unix (Linux/macOS), replacing a running executable on disk is fully supported
// because open executables hold open inode references.
func applyToExecutable(parentPID int, exePath, stagedPath string) error {
	if stagedPath == "" {
		return fmt.Errorf("%w: empty staged path", ErrUpdateNotReady)
	}

	stagedInfo, err := os.Stat(stagedPath)
	if err != nil {
		return fmt.Errorf("%w: staged update file not found at %s: %v", ErrUpdateNotReady, stagedPath, err)
	}
	if stagedInfo.Size() == 0 {
		return fmt.Errorf("%w: staged update file is empty", ErrUpdateNotReady)
	}

	// Ensure staged file has execution permissions
	if err := os.Chmod(stagedPath, 0o755); err != nil {
		return fmt.Errorf("apply failed: cannot set permissions on staged file: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	// Verify target directory is writable
	testFile := filepath.Join(exeDir, fmt.Sprintf(".write_test_%d", parentPID))
	if err := os.WriteFile(testFile, []byte{1}, 0o600); err != nil {
		return fmt.Errorf("apply failed: installation directory %s is not writable: %w", exeDir, err)
	}
	_ = os.Remove(testFile)

	// Stage the new binary in the destination directory to ensure that the
	// final replacement is on the same filesystem (avoiding EXDEV / cross-device link errors).
	newPath := exePath + ".new"
	_ = os.Remove(newPath)
	if err := moveFile(stagedPath, newPath); err != nil {
		return fmt.Errorf("apply failed: cannot stage new binary in destination directory: %w", err)
	}
	_ = os.Chmod(newPath, 0o755)

	backupPath := exePath + ".old"
	_ = os.Remove(backupPath)

	// 1. Move current running executable to backup
	if err := os.Rename(exePath, backupPath); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("apply failed: cannot backup current executable: %w", err)
	}

	// 2. Atomically place new executable at target location
	if err := os.Rename(newPath, exePath); err != nil {
		// ROLLBACK: restore previous executable
		_ = os.Rename(backupPath, exePath)
		_ = os.Remove(newPath)
		return fmt.Errorf("apply failed: cannot replace executable: %w", err)
	}

	_ = os.Chmod(exePath, 0o755)

	// 3. Remove backup
	_ = os.Remove(backupPath)

	logger.Infof("Successfully replaced binary at %s with staged update", exePath)

	// 4. Relaunch the updated executable in a detached session
	cmd := exec.Command(exePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logger.Warnf("Failed to relaunch application automatically: %v", err)
	} else {
		_ = cmd.Process.Release()
		logger.Infof("Relaunched updated application (new PID: %d)", cmd.Process.Pid)
	}

	return nil
}

func moveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// If cross-device move, fall back to copy + remove
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) && errors.Is(linkErr.Err, syscall.EXDEV) {
		if copyErr := copyFile(src, dst); copyErr != nil {
			return copyErr
		}
		_ = os.Remove(src)
		return nil
	}

	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return err
	}

	return out.Sync()
}
