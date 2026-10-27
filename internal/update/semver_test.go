package update

import (
	"testing"

	"obox-app/internal/testutil"
)

func TestParseSemVer(t *testing.T) {
	tests := []struct {
		input string
		want  SemVer
		err   bool
	}{
		{"1.2.3", SemVer{1, 2, 3}, false},
		{"1.2.3-rc.1", SemVer{1, 2, 3}, false},
		{"1.10.0", SemVer{1, 10, 0}, false},
		{"v1.2.3", SemVer{}, true},
		{"1.2", SemVer{}, true},
		{"invalid", SemVer{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseSemVer(tc.input)
			if tc.err {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
				testutil.ExpectedEqual(t, got, tc.want)
			}
		})
	}
}

func TestIsUpgrade(t *testing.T) {
	tests := []struct {
		current, candidate string
		want               bool
		wantErr            bool
	}{
		{"1.2.3", "1.2.4", true, false},
		{"1.9.9", "1.10.0", true, false},
		{"dev", "1.0.0", true, false},
		{"1.0.0-customer-xyz", "1.0.0", false, false},
		{"1.0.0-customer-xyz", "1.0.1", true, false},
		{"1.2.3", "1.2.3", false, false},
		{"1.2.4", "1.2.3", false, false},
		{"1.2.3", "invalid", false, true},
		{"v1.2.3", "1.2.3", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.current+"->"+tc.candidate, func(t *testing.T) {
			got, err := IsUpgrade(tc.current, tc.candidate)
			if tc.wantErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
				testutil.ExpectedEqual(t, got, tc.want)
			}
		})
	}
}
