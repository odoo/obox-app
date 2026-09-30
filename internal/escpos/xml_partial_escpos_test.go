package escpos

import (
	"bytes"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestParseXMLToRasterImage_WidthZeroNoPanic(t *testing.T) {
	// width="0" must not cause divide-by-zero panic
	payload := []byte(`<epos-print><image width="0">AQIDBA==</image></epos-print>`)
	data, err := ParseXMLToRasterImage(payload, 0)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(data) > 0)
}

func TestParseXMLToRasterImage_ExtremeDimensions(t *testing.T) {
	// Huge width/height must be capped/skipped safely without OOM
	payload := []byte(`<epos-print><image width="99999999" height="99999999">AQIDBA==</image></epos-print>`)
	data, err := ParseXMLToRasterImage(payload, 0)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(data) > 0)
}

func TestParseXMLToRasterImage_NewlinesAndRupee(t *testing.T) {
	// Multibyte characters (₹) and newlines (\n and &#10;) must not split or panic
	payload := []byte(`<epos-print><text>Total: ₹500&#10;Second Line&#10;Third Line</text></epos-print>`)
	data, err := ParseXMLToRasterImage(payload, 120)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(data) > 0)
	testutil.ExpectedEqual(t, data[0], byte(0x1d))
	testutil.ExpectedEqual(t, data[1], byte(0x76))
}

func TestParseXMLToRasterImage_PreservePadding(t *testing.T) {
	raw := "   Item Name              $10.00   "
	cleaned := cleanText(raw)
	testutil.ExpectedEqual(t, cleaned, "Item Name              $10.00")
}

func TestParseXMLToRasterImage_PulseOnly(t *testing.T) {
	payload := []byte(`<epos-print><pulse /></epos-print>`)
	data, err := ParseXMLToRasterImage(payload, 0)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, string(data), string(CmdPulse))
}

func TestParseXMLToRasterImage_PulseMixed(t *testing.T) {
	payload := []byte(`<epos-print><pulse /><text>Receipt with cash drawer kick</text></epos-print>`)
	data, err := ParseXMLToRasterImage(payload, 0)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(data) > 0)

	// Raster command GS v 0
	testutil.ExpectedEqual(t, data[0], byte(0x1d))
	testutil.ExpectedEqual(t, data[1], byte(0x76))
}

func TestParseXMLToRasterImage_Banding(t *testing.T) {
	var xmlBuf bytes.Buffer
	xmlBuf.WriteString(`<epos-print>`)
	for i := 0; i < 20; i++ {
		xmlBuf.WriteString(`<text>Long receipt line to create height</text><feed line="1" />`)
	}
	xmlBuf.WriteString(`</epos-print>`)

	data, err := ParseXMLToRasterImage(xmlBuf.Bytes(), 0)
	testutil.ExpectedNoError(t, err)

	// Count occurrences of GS v 0 (0x1d 0x76 0x30 0x00)
	gsV0 := []byte{0x1d, 0x76, 0x30, 0x00}
	count := bytes.Count(data, gsV0)
	testutil.ExpectedTrue(t, count >= 1, "Expected raster GS v 0 commands")
}

func TestExtractEPOSPrint_SharedHelper(t *testing.T) {
	valid := []byte(`leading text <epos-print><text>hello</text></epos-print> trailing text`)
	ep, err := ExtractEPOSPrint(valid)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, ep.Items, 1)

	invalid := []byte(`no root here`)
	_, err = ExtractEPOSPrint(invalid)
	testutil.ExpectedError(t, err)
}
