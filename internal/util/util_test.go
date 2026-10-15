package util

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"obox-app/internal/testutil"
)

func TestZipLogs_Success(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(sourceDir, 0755)
	testutil.ExpectedNoError(t, err)

	// Create test files
	file1 := filepath.Join(sourceDir, "app.log")
	file2 := filepath.Join(sourceDir, "error.log")
	content1 := "2026-08-12 Application started\n"
	content2 := "2026-08-12 No errors detected\n"

	err = os.WriteFile(file1, []byte(content1), 0644)
	testutil.ExpectedNoError(t, err)

	err = os.WriteFile(file2, []byte(content2), 0644)
	testutil.ExpectedNoError(t, err)

	targetZip := filepath.Join(tempDir, "exported-logs.zip")

	err = ZipLogs(sourceDir, targetZip)
	testutil.ExpectedNoError(t, err)

	// Verify zip file exists and can be extracted
	r, err := zip.OpenReader(targetZip)
	testutil.ExpectedNoError(t, err)
	defer r.Close()

	testutil.ExpectedLen(t, r.File, 2)

	foundMap := make(map[string]string)
	for _, f := range r.File {
		rc, err := f.Open()
		testutil.ExpectedNoError(t, err)
		data, err := io.ReadAll(rc)
		rc.Close()
		testutil.ExpectedNoError(t, err)
		foundMap[f.Name] = string(data)
	}

	testutil.ExpectedEqual(t, foundMap["app.log"], content1)
	testutil.ExpectedEqual(t, foundMap["error.log"], content2)
}

func TestZipLogs_EmptyDirectory_SourceDirNotFound_InvalidTargetZip(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "empty_logs")
	err := os.MkdirAll(sourceDir, 0755)
	testutil.ExpectedNoError(t, err)

	targetZip := filepath.Join(tempDir, "empty.zip")
	err = ZipLogs(sourceDir, targetZip)
	testutil.ExpectedNoError(t, err)

	r, err := zip.OpenReader(targetZip)
	testutil.ExpectedNoError(t, err)
	defer r.Close()

	testutil.ExpectedLen(t, r.File, 0)

	nonExistent := filepath.Join(tempDir, "non_existent_dir")
	targetZip = filepath.Join(tempDir, "output.zip")

	err = ZipLogs(nonExistent, targetZip)
	testutil.ExpectedErrorContains(t, err, "failed to read source directory")

	sourceDir = filepath.Join(tempDir, "logs")
	err = os.MkdirAll(sourceDir, 0755)
	testutil.ExpectedNoError(t, err)

	// Invalid target path: write into non-existent parent directory
	invalidTarget := filepath.Join(tempDir, "no_such_folder", "output.zip")

	err = ZipLogs(sourceDir, invalidTarget)
	testutil.ExpectedErrorContains(t, err, "failed to create zip file")
}

func TestEnableLinuxAutostart(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	err := EnableLinuxAutostart()
	testutil.ExpectedNoError(t, err)

	desktopFilePath := filepath.Join(tempDir, ".config", "autostart", "obox-app.desktop")
	data, err := os.ReadFile(desktopFilePath)
	testutil.ExpectedNoError(t, err)

	content := string(data)
	testutil.ExpectedContains(t, content, "[Desktop Entry]")
	testutil.ExpectedContains(t, content, "Type=Application")
	testutil.ExpectedContains(t, content, "Name=Obox App")
	testutil.ExpectedContains(t, content, "Exec=")
	testutil.ExpectedContains(t, content, "X-GNOME-Autostart-enabled=true")
}
