package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"obox-app/internal/testutil"
)

func TestApplyToExecutable_MissingFile(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "app")
	_ = os.WriteFile(exe, []byte("echo current"), 0o755)

	err := applyToExecutable(exe, filepath.Join(tempDir, "missing"))
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}

func TestApplyToExecutable_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "app")
	_ = os.WriteFile(exe, []byte("echo current"), 0o755)
	emptyStaged := filepath.Join(tempDir, "empty")
	_ = os.WriteFile(emptyStaged, []byte{}, 0o755)

	err := applyToExecutable(exe, emptyStaged)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}

func TestApplyToExecutable_Success(t *testing.T) {
	tempDir := t.TempDir()
	// Use path with space to verify space handling
	subDir := filepath.Join(tempDir, "dir with space")
	_ = os.MkdirAll(subDir, 0o755)

	exe := filepath.Join(subDir, "my app")
	_ = os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	staged := filepath.Join(tempDir, "staged")
	_ = os.WriteFile(staged, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	err := applyToExecutable(exe, staged)
	testutil.ExpectedNoError(t, err)

	// Wait up to 3 seconds for the updater script to execute replacement
	replaced := false
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		// If staged file was moved, replacement succeeded
		if _, err := os.Stat(staged); os.IsNotExist(err) {
			replaced = true
			break
		}
	}

	testutil.ExpectedTrue(t, replaced, "updater script did not replace the executable in time")
}
