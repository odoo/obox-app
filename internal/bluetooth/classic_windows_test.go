//go:build windows

package bluetooth

import (
	"encoding/binary"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestWindows_BTHAddressConversion(t *testing.T) {
	tests := []struct {
		name    string
		mac     string
		wantHex uint64
		wantErr bool
	}{
		{
			name:    "valid MAC",
			mac:     "00:11:22:33:44:55",
			wantHex: 0x001122334455,
			wantErr: false,
		},
		{
			name:    "valid MAC lowercase",
			mac:     "aa:bb:cc:dd:ee:ff",
			wantHex: 0xAABBCCDDEEFF,
			wantErr: false,
		},
		{
			name:    "invalid MAC length",
			mac:     "11:22:33:44:55",
			wantErr: true,
		},
		{
			name:    "invalid MAC char",
			mac:     "11:22:33:44:55:ZZ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := macToWindowsBTHAddr(tt.mac)
			if tt.wantErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
				testutil.ExpectedEqual(t, got, tt.wantHex)

				// Roundtrip test
				back := btAddrToMAC(got)
				normExpected, err := ValidateAddress(tt.mac)
				testutil.ExpectedNoError(t, err)
				testutil.ExpectedEqual(t, back, normExpected)
			}
		})
	}
}

func TestWindows_MakeSockaddrBTH(t *testing.T) {
	const addr uint64 = 0x001122334455
	const channel uint32 = 1

	sa := makeSockaddrBTH(addr, channel)

	family := binary.LittleEndian.Uint16(sa[0:2])
	testutil.ExpectedEqual(t, family, afBTH)

	gotAddr := binary.LittleEndian.Uint64(sa[2:10])
	testutil.ExpectedEqual(t, gotAddr, addr)

	gotCh := binary.LittleEndian.Uint32(sa[26:30])
	testutil.ExpectedEqual(t, gotCh, channel)
}

func TestWindows_CheckDependencies(t *testing.T) {
	deps := checkDependencies()
	testutil.ExpectedLen(t, deps, 0)
}

func TestWindows_IsPrinterDevice(t *testing.T) {
	// Case 1: Non-printer name without printer keyword
	testutil.ExpectedFalse(t, isPrinterDevice("Device1"), "Name without printer keyword should NOT identify device as printer")

	// Case 2: Name contains printer keyword
	testutil.ExpectedTrue(t, isPrinterDevice("POS Thermal Printer"), "Device with printer name should be identified as printer")

	// Case 3: Phone/Audio device — generic name
	testutil.ExpectedFalse(t, isPrinterDevice("My Headphones"), "Headphones should not be identified as printer")

	// Case 4: Name containing "scanner" — no printer keyword, so false
	testutil.ExpectedFalse(t, isPrinterDevice("Barcode Scanner"), "Scanner should not be identified as printer")
}

func TestWindows_IsPotentialPrinter(t *testing.T) {
	// Case 1: Name keyword match → isPrinterDevice fires, always included
	testutil.ExpectedTrue(t, isPotentialPrinter("POS Printer", 0x00040680), "Printer name should be included")

	// Case 2: Excluded major classes (Computer, Phone, Audio, Peripheral)
	testutil.ExpectedFalse(t, isPotentialPrinter("Laptop", 0x00000100), "Computer should be excluded")
	testutil.ExpectedFalse(t, isPotentialPrinter("iPhone", 0x00000200), "Phone should be excluded")
	testutil.ExpectedFalse(t, isPotentialPrinter("Headset", 0x00000400), "Audio device should be excluded")
	testutil.ExpectedFalse(t, isPotentialPrinter("Mouse", 0x00000500), "Peripheral device should be excluded")

	// Case 3: Imaging class (0x06) — ambiguous, kept for manual selection.
	// The old carve-out that excluded scanner-only Imaging devices is gone; we
	// no longer use CoD for inclusion, so the whole class passes through.
	testutil.ExpectedTrue(t, isPotentialPrinter("Scanner", 0x00000640), "Imaging-class device without printer name is kept for manual selection")

	// Case 4: Unknown / uncategorized device
	testutil.ExpectedTrue(t, isPotentialPrinter("Unknown Device", 0x00000000), "Unknown device should be included for manual selection")
}
