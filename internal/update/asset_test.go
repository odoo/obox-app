package update

import (
	"runtime"
	"strings"
	"testing"
)

func TestSelectAsset_MatchingPlatform(t *testing.T) {
	var currentAssetName string
	switch runtime.GOOS {
	case "linux":
		currentAssetName = assetNameLinux + "1.0.0"
	case "windows":
		currentAssetName = assetNameWindows + "1.0.0"
	case "darwin":
		currentAssetName = assetNameMacos + "1.0.0"
	default:
		t.Skip("unsupported test OS")
	}

	assets := []Asset{
		{Name: "other-unknown-file", DownloadURL: "https://github.com/test/other"},
		{Name: currentAssetName, DownloadURL: "https://github.com/test/current"},
		{Name: "SHA256SUMS", DownloadURL: "https://github.com/test/sums"},
	}

	selected, err := selectAsset(assets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Name != currentAssetName {
		t.Errorf("got %q, want %q", selected.Name, currentAssetName)
	}
}

func TestSelectAsset_MissingPlatformAsset(t *testing.T) {
	assets := []Asset{
		{Name: "unrelated-file-pkg", DownloadURL: "https://github.com/test/unrelated"},
	}

	_, err := selectAsset(assets)
	if err == nil {
		t.Fatal("expected error when platform asset missing, got nil")
	}
}

func TestSelectAsset_Empty(t *testing.T) {
	_, err := selectAsset(nil)
	if err == nil {
		t.Fatal("expected error when assets slice is empty, got nil")
	}
}

func TestParseChecksums(t *testing.T) {
	raw := `
# Release checksums
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  obox-app-linux64-v1.0.17
a9f621fa06c701e541266dc9d81adcf267419301e41135c9ccbf8efc12e83eb8 *obox-app-win64-installer-v1.0.17.exe
`
	sums, err := parseChecksums(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseChecksums error: %v", err)
	}

	if got := sums["obox-app-linux64-v1.0.17"]; got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("linux checksum mismatch, got %q", got)
	}
	if got := sums["obox-app-win64-installer-v1.0.17.exe"]; got != "a9f621fa06c701e541266dc9d81adcf267419301e41135c9ccbf8efc12e83eb8" {
		t.Errorf("win checksum mismatch, got %q", got)
	}
}

func TestValidateDownloadURL(t *testing.T) {
	valid := []string{
		"https://github.com/djip-odoo/obox-app/releases/download/v1.0.17/obox-app-linux64-v1.0.17",
		"https://api.github.com/repos/djip-odoo/obox-app/releases/assets/123",
		"https://raw.github.com/test",
	}
	for _, u := range valid {
		if err := ValidateDownloadURL(u); err != nil {
			t.Errorf("ValidateDownloadURL(%q) unexpected error: %v", u, err)
		}
	}

	invalid := []string{
		"https://evil.com/malware.exe",
		"https://github.com.attacker.com/malware.exe",
		"ftp://github.com/test",
		"http://github.com/insecure",
		"http://127.0.0.1:45455/pkg.bin",
		"http://localhost:8080/pkg.bin",
		"https://objects.githubusercontent.com/github-production-release-asset-2e65be/123",
	}
	for _, u := range invalid {
		if err := ValidateDownloadURL(u); err == nil {
			t.Errorf("ValidateDownloadURL(%q) expected error, got nil", u)
		}
	}
}

func TestFetchExpectedChecksum_Digest(t *testing.T) {
	asset := &Asset{
		Name:   "obox-app-linux64-1.0.17",
		Digest: "sha256:70e3d6e17c9f8843bec55923f5743081b4963ea6e0fcf8f051bfdbc8d2ed5418",
	}

	sha, err := FetchExpectedChecksum(asset, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "70e3d6e17c9f8843bec55923f5743081b4963ea6e0fcf8f051bfdbc8d2ed5418"
	if sha != expected {
		t.Errorf("got %q, want %q", sha, expected)
	}
}
