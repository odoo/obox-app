package printer

import (
	"testing"

	"obox-app/internal/testutil"
)

func TestGetPrinterType(t *testing.T) {
	tests := []struct {
		vidPid   string
		expected Type
	}{
		// Known receipt printers
		{"2aaf:6015", TypeReceipt},
		{"2AAF:6015", TypeReceipt}, // case-insensitive
		{"04b8:0e32", TypeReceipt},
		{"04B8:0202", TypeReceipt},
		{"04b8:0e27", TypeReceipt},
		{"0483:5720", TypeReceipt},
		{"2d84:c7c8", TypeReceipt},
		{"4b43:3830", TypeReceipt},

		// Known label printers
		{"0a5f:0187", TypeLabel},
		{"0A5F:0187", TypeLabel}, // case-insensitive
		{"195f:0001", TypeLabel},

		// Unknown VID:PID -> defaults to TypeReceipt
		{"1234:5678", TypeReceipt},
		{"", TypeReceipt},
	}

	for _, tc := range tests {
		got := getPrinterType(tc.vidPid)
		testutil.ExpectedEqual(t, got, tc.expected)
	}
}

func TestIsKnownPrinter(t *testing.T) {
	// Epson printer 04b8:0202
	epsonDesc := testutil.MockEpsonPrinterDesc()
	testutil.ExpectedTrue(t, isKnownPrinter(epsonDesc))

	// Zebra printer 0a5f:0187
	zebraDesc := testutil.MockZebraPrinterDesc()
	testutil.ExpectedTrue(t, isKnownPrinter(zebraDesc))

	// Non-printer mass storage 1234:5678
	storageDesc := testutil.MockMassStorageDesc()
	testutil.ExpectedFalse(t, isKnownPrinter(storageDesc))
}
