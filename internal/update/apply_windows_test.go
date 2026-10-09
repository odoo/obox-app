//go:build windows

package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestApply_MissingFile(t *testing.T) {
	windowsApplyMu.Lock()
	installerStarted = false
	windowsApplyMu.Unlock()

	nonExistent := filepath.Join(t.TempDir(), "nonexistent-installer.exe")
	err := Apply(nonExistent)
	if err == nil || !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("expected ErrUpdateNotReady for missing installer, got %v", err)
	}
}

func TestApply_EmptyPath(t *testing.T) {
	windowsApplyMu.Lock()
	installerStarted = false
	windowsApplyMu.Unlock()

	err := Apply("")
	if err == nil || !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("expected ErrUpdateNotReady for empty path, got %v", err)
	}
}

func TestApply_EmptyFile(t *testing.T) {
	windowsApplyMu.Lock()
	installerStarted = false
	windowsApplyMu.Unlock()

	emptyFile := filepath.Join(t.TempDir(), "empty-installer.exe")
	if err := os.WriteFile(emptyFile, []byte{}, 0o755); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	err := Apply(emptyFile)
	if err == nil || !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("expected ErrUpdateNotReady for empty installer file, got %v", err)
	}
}

func TestApply_Success(t *testing.T) {
	installerFile := filepath.Join(t.TempDir(), "valid-installer.exe")
	if err := os.WriteFile(installerFile, []byte("dummy installer binary"), 0o755); err != nil {
		t.Fatalf("failed to create dummy installer file: %v", err)
	}

	windowsApplyMu.Lock()
	installerStarted = false
	oldShellExecute := shellExecute
	windowsApplyMu.Unlock()

	var executedFile string
	shellExecute = func(hwnd windows.Handle, operation, file, args, dir *uint16, showCmd int32) error {
		executedFile = windows.UTF16PtrToString(file)
		return nil
	}

	t.Cleanup(func() {
		windowsApplyMu.Lock()
		installerStarted = false
		shellExecute = oldShellExecute
		windowsApplyMu.Unlock()
	})

	err := Apply(installerFile)
	if err != nil {
		t.Fatalf("expected nil error on successful apply, got: %v", err)
	}
	if executedFile != installerFile {
		t.Errorf("expected shellExecute to be called with %q, got %q", installerFile, executedFile)
	}

	// Calling Apply again should fail with ErrUpdateInProgress
	err2 := Apply(installerFile)
	if err2 == nil || !errors.Is(err2, ErrUpdateInProgress) {
		t.Errorf("expected ErrUpdateInProgress on repeated apply, got: %v", err2)
	}
}

func TestApply_LaunchError(t *testing.T) {
	installerFile := filepath.Join(t.TempDir(), "failing-installer.exe")
	if err := os.WriteFile(installerFile, []byte("dummy installer binary"), 0o755); err != nil {
		t.Fatalf("failed to create dummy installer file: %v", err)
	}

	windowsApplyMu.Lock()
	installerStarted = false
	oldShellExecute := shellExecute
	windowsApplyMu.Unlock()

	shellExecute = func(hwnd windows.Handle, operation, file, args, dir *uint16, showCmd int32) error {
		return errors.New("access denied")
	}

	t.Cleanup(func() {
		windowsApplyMu.Lock()
		installerStarted = false
		shellExecute = oldShellExecute
		windowsApplyMu.Unlock()
	})

	err := Apply(installerFile)
	if err == nil || !strings.Contains(err.Error(), "apply failed: cannot launch installer") {
		t.Errorf("expected launch failure error, got: %v", err)
	}

	windowsApplyMu.Lock()
	started := installerStarted
	windowsApplyMu.Unlock()
	if started {
		t.Error("installerStarted should remain false when launch fails")
	}
}
