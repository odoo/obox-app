package update

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"obox-app/internal/testutil"
)

func TestSelectAsset(t *testing.T) {
	name := fmt.Sprintf("obox-app-%s-%s-1.0.0", runtime.GOOS, runtime.GOARCH)

	tests := []struct {
		name    string
		assets  []Asset
		want    string
		wantErr bool
	}{
		{
			name: "matching platform",
			assets: []Asset{
				{Name: "SHA256SUMS"},
				{Name: name + ".sha256"},
				{Name: name},
			},
			want: name,
		},
		{
			name:    "missing platform",
			assets:  []Asset{{Name: "unrelated-file-pkg"}},
			wantErr: true,
		},
		{
			name:    "empty assets",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SelectAsset(tc.assets)
			if tc.wantErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
				testutil.ExpectedEqual(t, got.Name, tc.want)
			}
		})
	}
}

func TestParseChecksums(t *testing.T) {
	raw := `# Release checksums
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  obox-app-linux64-v1.0.17
a9f621fa06c701e541266dc9d81adcf267419301e41135c9ccbf8efc12e83eb8 *obox-app-win64-installer-v1.0.17.exe`

	got, err := parseChecksums(strings.NewReader(raw))
	testutil.ExpectedNoError(t, err)

	for name, want := range map[string]string{
		"obox-app-linux64-v1.0.17":             "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"obox-app-win64-installer-v1.0.17.exe": "a9f621fa06c701e541266dc9d81adcf267419301e41135c9ccbf8efc12e83eb8",
	} {
		testutil.ExpectedEqual(t, got[name], want)
	}
}

func TestValidateDownloadURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://github.com/djip-odoo/obox-app/releases/download/v1.0.17/asset", false},
		{"https://api.github.com/repos/djip-odoo/obox-app/releases/assets/123", false},
		{"https://raw.github.com/test", false},
		{"https://objects.githubusercontent.com/123", false},
		{"https://github-releases.githubusercontent.com/123", false},
		{"https://evil.com/malware.exe", true},
		{"https://github.com.attacker.com/malware.exe", true},
		{"https://githubusercontent.com.attacker.com/malware.exe", true},
		{"ftp://github.com/test", true},
		{"http://github.com/insecure", true},
		{"http://127.0.0.1:45455/pkg.bin", true},
		{"http://localhost:8080/pkg.bin", true},
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			err := ValidateDownloadURL(tc.url)
			if tc.wantErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
			}
		})
	}
}

func TestFetchExpectedChecksum_Digest(t *testing.T) {
	asset := &Asset{
		Name:   "obox-app-linux64-1.0.17",
		Digest: "sha256:70e3d6e17c9f8843bec55923f5743081b4963ea6e0fcf8f051bfdbc8d2ed5418",
	}

	got, err := FetchExpectedChecksum(context.Background(), asset, nil)
	testutil.ExpectedNoError(t, err)

	want := "70e3d6e17c9f8843bec55923f5743081b4963ea6e0fcf8f051bfdbc8d2ed5418"
	testutil.ExpectedEqual(t, got, want)
}
