//go:build windows

package update

import (
	"errors"
	"path/filepath"
	"testing"

	"obox-app/internal/testutil"
)

func TestApply_MissingFile(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "nonexistent-installer.exe")
	err := applyStaged(nonExistent)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}

func TestApply_EmptyPath(t *testing.T) {
	err := applyStaged("")
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}
