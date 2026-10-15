package escpos

import (
	"bytes"
	"encoding/base64"
	"testing"

	"obox-app/internal/testutil"
)

func TestParseXML_MissingRoot_MalformedXML(t *testing.T) {
	_, err := ParseXML([]byte("<invalid>text</invalid>"))
	testutil.ExpectedErrorContains(t, err, "no <epos-print> element found")

	_, err = ParseXML([]byte("<epos-print><text>unclosed"))
	testutil.ExpectedErrorContains(t, err, "no <epos-print> element found")

	_, err = ParseXML([]byte("<epos-print><text>unclosed</epos-print>"))
	testutil.ExpectedErrorContains(t, err, "XML parse error")
}

func TestParseXML_TextElement(t *testing.T) {
	xml := `<epos-print xmlns="http://www.epson-pos.com/schemas/2011/03/epos-print">
		<text align="center" font="font_a" em="true" ul="true" dw="true" dh="true">Sample Text</text>
	</epos-print>`

	got, err := ParseXML([]byte(xml))
	testutil.ExpectedNoError(t, err)

	expectedPrefix := []byte{
		ESC, 0x40, // CmdInit
		ESC, 0x61, 0x01, // align center
		GS, 0x21, 0x11, // dw + dh
		ESC, 0x4D, 0x00, // font_a
		ESC, 0x45, 0x01, // em true
		ESC, 0x2D, 0x01, // ul true
	}
	expected := append(expectedPrefix, []byte("Sample Text")...)
	testutil.ExpectedBytesEqual(t, got, expected)
}

func TestParseXML_FeedElement(t *testing.T) {
	// Normal feed (3 lines)
	xml := `<epos-print><feed line="3" /></epos-print>`
	got, err := ParseXML([]byte(xml))
	testutil.ExpectedNoError(t, err)
	expected := []byte{ESC, 0x40, LF, LF, LF}
	testutil.ExpectedBytesEqual(t, got, expected)

	// Default line count when line attr is missing (defaults to 1)
	xmlDefault := `<epos-print><feed /></epos-print>`
	gotDefault, err := ParseXML([]byte(xmlDefault))
	testutil.ExpectedNoError(t, err)
	expectedDefault := []byte{ESC, 0x40, LF}
	testutil.ExpectedBytesEqual(t, gotDefault, expectedDefault)

	// Clamp lower bound (0 -> clamped to 1)
	xmlClampLow := `<epos-print><feed line="0" /></epos-print>`
	gotClampLow, err := ParseXML([]byte(xmlClampLow))
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedBytesEqual(t, gotClampLow, expectedDefault)

	// Clamp upper bound (300 -> clamped to 255)
	xmlClampHigh := `<epos-print><feed line="300" /></epos-print>`
	gotClampHigh, err := ParseXML([]byte(xmlClampHigh))
	testutil.ExpectedNoError(t, err)
	// CmdInit (2 bytes) + 255 LFs
	testutil.ExpectedEqual(t, len(gotClampHigh), 2+255)

	// Invalid string for line attribute (should default to 1)
	xmlInvalid := `<epos-print><feed line="abc" /></epos-print>`
	gotInvalid, err := ParseXML([]byte(xmlInvalid))
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedBytesEqual(t, gotInvalid, expectedDefault)
}

func TestParseXML_CutAndPulse(t *testing.T) {
	xml := `<epos-print>
		<cut />
		<pulse />
	</epos-print>`

	got, err := ParseXML([]byte(xml))
	testutil.ExpectedNoError(t, err)

	expected := append([]byte(nil), CmdInit...)
	expected = append(expected, CmdCut...)
	expected = append(expected, CmdPulse...)
	testutil.ExpectedBytesEqual(t, got, expected)
}

func TestParseXML_ImageElement(t *testing.T) {
	bitmap := []byte{0xAA, 0x55}
	b64 := base64.StdEncoding.EncodeToString(bitmap)

	xml := `<epos-print>
		<image align="center" width="8" height="2">` + b64 + `</image>
	</epos-print>`

	got, err := ParseXML([]byte(xml))
	testutil.ExpectedNoError(t, err)

	expectedPrefix := []byte{
		ESC, 0x40, // CmdInit
		ESC, 0x61, 0x01, // align center
		GS, 0x76, 0x30, 0x00,
		0x01, 0x00, // xL=1, xH=0
		0x02, 0x00, // yL=2, yH=0
	}
	expected := append(expectedPrefix, bitmap...)
	testutil.ExpectedBytesEqual(t, got, expected)
}

func TestParseXML_ImageElement_Error(t *testing.T) {
	xml := `<epos-print>
		<image width="8" height="2">invalid-base64!</image>
	</epos-print>`

	_, err := ParseXML([]byte(xml))
	testutil.ExpectedErrorContains(t, err, "image element")
}

func TestParseXML_UnsupportedElement(t *testing.T) {
	xml := `<epos-print>
		<barcode>123456</barcode>
	</epos-print>`

	_, err := ParseXML([]byte(xml))
	testutil.ExpectedErrorContains(t, err, "unsupported element <barcode>")
}

func TestParseXML_CompleteDocument(t *testing.T) {
	xml := `<epos-print>
		<text align="center" em="true">RECEIPT</text>
		<feed line="1" />
		<text align="left">Item 1: $10.00</text>
		<feed line="2" />
		<cut />
		<pulse />
	</epos-print>`

	got, err := ParseXML([]byte(xml))
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(got) > 0, "Expected non-empty output for complete document")
	testutil.ExpectedTrue(t, bytes.HasPrefix(got, CmdInit), "Expected output to start with CmdInit")
	testutil.ExpectedTrue(t, bytes.Contains(got, CmdCut), "Expected output to contain CmdCut")
	testutil.ExpectedTrue(t, bytes.Contains(got, CmdPulse), "Expected output to contain CmdPulse")
}

func TestHelper_BoolPtr(t *testing.T) {
	m := map[string]string{
		"k1": "1",
		"k2": "true",
		"k3": "0",
		"k4": "false",
		"k5": "other",
	}

	p1 := boolPtr(m, "k1")
	testutil.ExpectedNotNil(t, p1)
	testutil.ExpectedTrue(t, *p1)

	p2 := boolPtr(m, "k2")
	testutil.ExpectedNotNil(t, p2)
	testutil.ExpectedTrue(t, *p2)

	p3 := boolPtr(m, "k3")
	testutil.ExpectedNotNil(t, p3)
	testutil.ExpectedFalse(t, *p3)

	p4 := boolPtr(m, "k4")
	testutil.ExpectedNotNil(t, p4)
	testutil.ExpectedFalse(t, *p4)

	p5 := boolPtr(m, "k5")
	testutil.ExpectedNil(t, p5)

	pNonExistent := boolPtr(m, "nonexistent")
	testutil.ExpectedNil(t, pNonExistent)
}

func TestHelper_Clamp(t *testing.T) {
	testutil.ExpectedEqual(t, clamp(5, 1, 10), 5)
	testutil.ExpectedEqual(t, clamp(0, 1, 10), 1)
	testutil.ExpectedEqual(t, clamp(15, 1, 10), 10)
}
