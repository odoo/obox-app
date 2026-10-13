//go:build windows

package update

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestApply_MissingFile(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "nonexistent-installer.exe")
	err := Apply(nonExistent)
	if err == nil || !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("expected ErrUpdateNotReady for missing installer, got %v", err)
	}
}

func TestApply_EmptyPath(t *testing.T) {
	err := Apply("")
	if err == nil || !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("expected ErrUpdateNotReady for empty path, got %v", err)
	}
}
