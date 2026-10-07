package printer

import (
	"errors"
	"testing"

	"obox-app/internal/testutil"
)

func TestEncodePrinterID(t *testing.T) {
	// Case 1: Serial and VidPid provided -> encodes both vp:<vidpid> and s:<serial>
	pWithSerial := &LibUsbPrinter{
		Serial: "SN123456",
		VidPid: "04B8:0202",
		Path:   "1.2.3",
	}
	encoded, err := encodePrinterID(pWithSerial)
	testutil.ExpectedNoError(t, err)

	decoded, err := decodePrinterID(encoded)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, decoded.Serial, "SN123456")
	testutil.ExpectedEqual(t, decoded.VidPid, "04B8:0202")
	testutil.ExpectedEqual(t, decoded.Path, "")

	// Case 2: Only serial provided (no VidPid, no Path)
	pOnlySerial := &LibUsbPrinter{Serial: "SN987654"}
	encodedOnlySerial, err := encodePrinterID(pOnlySerial)
	testutil.ExpectedNoError(t, err)

	decodedOnlySerial, err := decodePrinterID(encodedOnlySerial)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, decodedOnlySerial.Serial, "SN987654")
	testutil.ExpectedEqual(t, decodedOnlySerial.VidPid, "")
	testutil.ExpectedEqual(t, decodedOnlySerial.Path, "")

	// Case 3: No serial, but VidPid and Path provided
	pNoSerial := &LibUsbPrinter{
		VidPid: "04B8:0202",
		Path:   "1.2.3",
	}
	encoded2, err := encodePrinterID(pNoSerial)
	testutil.ExpectedNoError(t, err)

	decoded2, err := decodePrinterID(encoded2)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, decoded2.Serial, "")
	testutil.ExpectedEqual(t, decoded2.VidPid, "04B8:0202")
	testutil.ExpectedEqual(t, decoded2.Path, "1.2.3")

	// Case 4: Completely empty LibUsbPrinter -> should return error
	pEmpty := &LibUsbPrinter{}
	_, err = encodePrinterID(pEmpty)
	testutil.ExpectedError(t, err)
}

func TestDecodePrinterID_Invalid(t *testing.T) {
	// Invalid base64
	_, err := decodePrinterID("not-valid-base64!!!")
	testutil.ExpectedTrue(t, errors.Is(err, ErrInvalidPrinterID))

	// Valid base64 but empty payload
	_, err = decodePrinterID("")
	testutil.ExpectedTrue(t, errors.Is(err, ErrInvalidPrinterID))
}

func TestLANPrinterID_Roundtrip(t *testing.T) {
	ip := "192.168.1.150"
	encoded := EncodeLANPrinterID(ip)

	decoded, ok := DecodeLANPrinterID(encoded)
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, decoded, ip)
}

func TestDecodeLANPrinterID_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid base64", "!!!bad-base64"},
		{"empty string", ""},
		{"too short", "bA"},                    // decoded length < 3
		{"missing colon", "bHh4"},              // decoded "lxx"
		{"wrong prefix", "dToxOTIuMTY4LjEuMQ"}, // decoded "u:192.168.1.1"
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := DecodeLANPrinterID(tc.input)
			testutil.ExpectedFalse(t, ok)
		})
	}
}
