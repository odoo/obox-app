//go:build !windows

package update

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyToExecutable_MissingFile(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "app")
	_ = os.WriteFile(exe, []byte("echo current"), 0o755)

	err := applyToExecutable(os.Getpid(), exe, filepath.Join(tempDir, "missing"))
	if err == nil || !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("expected ErrUpdateNotReady, got %v", err)
	}
}

func TestApplyToExecutable_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "app")
	_ = os.WriteFile(exe, []byte("echo current"), 0o755)
	emptyStaged := filepath.Join(tempDir, "empty")
	_ = os.WriteFile(emptyStaged, []byte{}, 0o755)

	err := applyToExecutable(os.Getpid(), exe, emptyStaged)
	if err == nil || !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("expected ErrUpdateNotReady, got %v", err)
	}
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

	// Start a short-lived process and use its PID
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skip("cannot start mock process:", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait() // already finished, so updater won't block waiting

	err := applyToExecutable(pid, exe, staged)
	if err != nil {
		t.Fatalf("ApplyToExecutable error: %v", err)
	}

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

	if !replaced {
		t.Error("updater script did not replace the executable in time")
	}
}
