package printer

import (
	"testing"

	"epos-proxy/internal/testutil"
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

func TestGetPrinterProtocol(t *testing.T) {
	tests := []struct {
		vidPid   string
		expected Protocol
	}{
		// Standard ESCPOS printers
		{"2aaf:6015", ProtocolESCPOS},
		{"2AAF:6015", ProtocolESCPOS}, // case-insensitive
		{"04b8:0e32", ProtocolESCPOS},
		{"04b8:0202", ProtocolESCPOS},
		{"04b8:0e27", ProtocolESCPOS},
		{"2d84:c7c8", ProtocolESCPOS},
		{"4b43:3830", ProtocolESCPOS},

		// ESCPOS_PARTIAL printers
		{"0483:5720", ProtocolESCPOSPartial},
		{"0483:5720", ProtocolESCPOSPartial}, // case-insensitive
		{"0456:0808", ProtocolESCPOSPartial},

		// Label printers (no ESCPOS protocol set)
		{"0a5f:0187", ""},
		{"195f:0001", ""},

		// Unknown VID:PID -> defaults to ProtocolESCPOS
		{"1234:5678", ProtocolESCPOS},
		{"", ProtocolESCPOS},
	}

	for _, tc := range tests {
		got := getKnownPrinter(tc.vidPid).Protocol
		testutil.ExpectedEqual(t, got, tc.expected)
	}
}

func TestGetPrinterProtocolByID(t *testing.T) {
	// USB printer ID containing partial ESCPOS printer VID:PID
	encodedID, err := encodePrinterID(&LibUsbPrinter{
		VidPid: "0483:5720",
		Serial: "12345",
	})
	proto, _ := getKnownPrinterByID(encodedID)
	testutil.ExpectedEqual(t, proto, ProtocolESCPOSPartial)

	// USB printer ID containing standard ESCPOS printer VID:PID
	encodedIDStandard, err := encodePrinterID(&LibUsbPrinter{
		VidPid: "04b8:0202",
		Serial: "67890",
	})
	testutil.ExpectedNoError(t, err)
	protoStandard, _ := getKnownPrinterByID(encodedIDStandard)
	testutil.ExpectedEqual(t, protoStandard, ProtocolESCPOS)

	// Unknown or invalid ID -> defaults to ProtocolESCPOS
	protoInvalid, _ := getKnownPrinterByID("invalid")
	testutil.ExpectedEqual(t, protoInvalid, ProtocolESCPOS)
}

func TestParseProtocol(t *testing.T) {
	testutil.ExpectedEqual(t, ParseProtocol(""), ProtocolESCPOS)
	testutil.ExpectedEqual(t, ParseProtocol("unknown"), ProtocolESCPOS)
	testutil.ExpectedEqual(t, ParseProtocol("ESCPOS"), ProtocolESCPOS)
	testutil.ExpectedEqual(t, ParseProtocol("escpos"), ProtocolESCPOS)
	testutil.ExpectedEqual(t, ParseProtocol("ESCPOS_PARTIAL"), ProtocolESCPOSPartial)
	testutil.ExpectedEqual(t, ParseProtocol("escpos_partial"), ProtocolESCPOSPartial)
}
