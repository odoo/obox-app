package update

import (
	"testing"
)

func TestParseSemVer(t *testing.T) {
	tests := []struct {
		input     string
		wantErr   bool
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{"1.2.3", false, 1, 2, 3},
		{"v1.2.3", true, 0, 0, 0},
		{"V1.2.3", true, 0, 0, 0},
		{"1.10.0", false, 1, 10, 0},
		{"1.0.0-customer-xyz", false, 1, 0, 0},
		{"1.2.3-rc.1", false, 1, 2, 3},
		{"1.2.3-beta.2+build.42", false, 1, 2, 3},
		{"", true, 0, 0, 0},
		{"invalid", true, 0, 0, 0},
		{"1.2", true, 0, 0, 0},
		{"1.2.3.4", true, 0, 0, 0},
		{"v", true, 0, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			v, err := ParseSemVer(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseSemVer(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSemVer(%q) unexpected error: %v", tc.input, err)
			}
			if v.Major != tc.wantMajor || v.Minor != tc.wantMinor || v.Patch != tc.wantPatch {
				t.Errorf("got %d.%d.%d, want %d.%d.%d", v.Major, v.Minor, v.Patch, tc.wantMajor, tc.wantMinor, tc.wantPatch)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		// Basic ordering
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.9.9", "1.10.0", -1},
		{"1.10.0", "1.9.9", 1},

		// Suffixes trimmed to base standard version
		{"1.0.0-customer-xyz", "1.0.0", 0},
		{"1.0.0-customer-xyz", "1.0.1", -1},
		{"1.2.3-rc.1", "1.2.3", 0},
		{"1.2.3-beta", "1.2.3", 0},

		// Development versions
		{"dev", "1.0.0", -1},
		{"local", "1.0.0", -1},
		{"", "1.0.0", -1},
	}

	for _, tc := range tests {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			got, err := compareVersions(tc.a, tc.b)
			if err != nil {
				t.Fatalf("CompareVersions(%q, %q) error: %v", tc.a, tc.b, err)
			}
			if got != tc.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestCompareVersions_Invalid(t *testing.T) {
	if _, err := compareVersions("1.0.0", "invalid"); err == nil {
		t.Error("expected error for invalid candidate version")
	}
	if _, err := compareVersions("1.0.0", "dev"); err == nil {
		t.Error("expected error for dev candidate version")
	}
	if _, err := compareVersions("invalid", "1.0.0"); err == nil {
		t.Error("expected error for invalid current version")
	}
	if _, err := compareVersions("1.0.0", "v1.2.3"); err == nil {
		t.Error("expected error for v-prefixed candidate version")
	}
	if _, err := compareVersions("v1.2.3", "1.0.0"); err == nil {
		t.Error("expected error for v-prefixed current version")
	}
}

func TestIsUpgrade(t *testing.T) {
	tests := []struct {
		current   string
		candidate string
		want      bool
		wantErr   bool
	}{
		{"1.2.3", "1.2.4", true, false},
		{"1.9.9", "1.10.0", true, false},
		{"dev", "1.0.0", true, false},
		{"local", "1.0.0", true, false},

		// Customer custom hotfix build -> should not overwrite same base release, but upgrades to next
		{"1.0.0-customer-xyz", "1.0.0", false, false},
		{"1.0.0-customer-xyz", "1.0.1", true, false},

		// Same version -> not an upgrade
		{"1.2.3", "1.2.3", false, false},

		// Downgrade -> not an upgrade
		{"1.2.4", "1.2.3", false, false},
		{"1.10.0", "1.9.9", false, false},

		// Malformed -> error
		{"1.2.3", "invalid", false, true},
		{"1.2.3", "v1.2.3", false, true},
		{"v1.2.3", "1.2.3", false, true},
		{"1.2.3", "", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.current+"->"+tc.candidate, func(t *testing.T) {
			got, err := IsUpgrade(tc.current, tc.candidate)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IsUpgrade(%q, %q) error = %v, wantErr = %v", tc.current, tc.candidate, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("IsUpgrade(%q, %q) = %v, want %v", tc.current, tc.candidate, got, tc.want)
			}
		})
	}
}
