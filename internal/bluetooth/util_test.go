package bluetooth

import (
	"testing"

	"epos-proxy/internal/testutil"
)

func TestParseMACToBytes(t *testing.T) {
	tests := []struct {
		name      string
		mac       string
		want      [6]byte
		expectErr bool
	}{
		{
			name:      "valid standard MAC",
			mac:       "11:22:33:44:55:66",
			want:      [6]byte{0x66, 0x55, 0x44, 0x33, 0x22, 0x11}, // reversed little-endian
			expectErr: false,
		},
		{
			name:      "valid lowercase MAC",
			mac:       "aa:bb:cc:dd:ee:ff",
			want:      [6]byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa},
			expectErr: false,
		},
		{
			name:      "invalid octet count",
			mac:       "11:22:33:44:55",
			want:      [6]byte{},
			expectErr: true,
		},
		{
			name:      "invalid hex characters",
			mac:       "11:22:33:44:55:ZZ",
			want:      [6]byte{},
			expectErr: true,
		},
		{
			name:      "empty string",
			mac:       "",
			want:      [6]byte{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMACToBytes(tt.mac)
			if tt.expectErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
				testutil.ExpectedBytesEqual(t, got[:], tt.want[:])
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
	valid := []struct {
		input string
		want  string
	}{
		{input: "AA:BB:CC:DD:EE:FF", want: "AA:BB:CC:DD:EE:FF"},
		{input: "aa:bb:cc:dd:ee:ff", want: "AA:BB:CC:DD:EE:FF"},
		{input: "AA-BB-CC-DD-EE-FF", want: "AA:BB:CC:DD:EE:FF"},
		{input: "aa-bb-cc-dd-ee-ff", want: "AA:BB:CC:DD:EE:FF"},
		{input: "  11:22:33:44:55:66  ", want: "11:22:33:44:55:66"},
		{input: "00:11:22:33:44:55", want: "00:11:22:33:44:55"},
		{input: "00001800-0000-1000-8000-00805f9b34fb", want: "00001800-0000-1000-8000-00805F9B34FB"},
		{input: "  e2c56db5-dffb-48d2-b060-d0f5a71096e0  ", want: "E2C56DB5-DFFB-48D2-B060-D0F5A71096E0"},
	}

	for _, tc := range valid {
		t.Run("valid_"+tc.input, func(t *testing.T) {
			got, err := ValidateAddress(tc.input)
			testutil.ExpectedNoError(t, err)
			testutil.ExpectedEqual(t, got, tc.want)
		})
	}

	invalid := []string{
		"",
		"   ",
		"invalid-mac",
		"11:22:33:44:55",
		"11:22:33:44:55:66:77",
		"GG:HH:II:JJ:KK:LL",
		"00001800-0000-1000-8000-00805f9b34f",
	}

	for _, addr := range invalid {
		t.Run("invalid_"+addr, func(t *testing.T) {
			_, err := ValidateAddress(addr)
			testutil.ExpectedError(t, err)
		})
	}
}
