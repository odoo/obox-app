package printer

import (
	"testing"

	"obox-app/internal/testutil"
)

func TestSanitizeDeviceID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "contains null bytes and control chars", input: "MFG:EPSON\x00;MDL:\x01TM-T20II\x00;\x1f", expected: "MFG:EPSON;MDL:TM-T20II;"},
		{name: "unicode printables preserved", input: "MFG:ÜberPrinter;", expected: "MFG:ÜberPrinter;"},
		{name: "plain ascii", input: "MFG:EPSON;MDL:TM-T20II;", expected: "MFG:EPSON;MDL:TM-T20II;"},
		{name: "empty string", input: "", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeDeviceID(tc.input)
			testutil.ExpectedEqual(t, got, tc.expected)
		})
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CMD", "CMD"},
		{"command set", "CMD"},
		{"COMMANDSET", "CMD"},
		{"command", "CMD"},
		{"commands", "CMD"},
		{"MFG", "MFG"},
		{"manufacturer", "MFG"},
		{"MDL", "MDL"},
		{"model", "MDL"},
		{"CLS", "CLS"},
		{"class", "CLS"},
		{"custom_key", "CUSTOM_KEY"},
		{"  mfg  ", "MFG"},
	}

	for _, tc := range tests {
		got := normalizeKey(tc.input)
		testutil.ExpectedEqual(t, got, tc.expected)
	}
}

func TestParseDeviceID(t *testing.T) {
	raw := "MFG:EPSON;CMD:ESC/POS;MDL:TM-T20II;CLS:PRINTER;DESCRIPTION:Receipt Printer;"
	got := parseDeviceID(raw)

	testutil.ExpectedEqual(t, got["MFG"], "EPSON")
	testutil.ExpectedEqual(t, got["CMD"], "ESC/POS")
	testutil.ExpectedEqual(t, got["MDL"], "TM-T20II")
	testutil.ExpectedEqual(t, got["CLS"], "PRINTER")
	testutil.ExpectedEqual(t, got["DESCRIPTION"], "Receipt Printer")
}

func TestParseDeviceID_DuplicateKeys(t *testing.T) {
	// Aliases map "COMMAND" and "CMD" both to "CMD" -> should be concatenated with comma
	raw := "CMD:ESC/POS;COMMAND:STAR;"
	got := parseDeviceID(raw)
	testutil.ExpectedEqual(t, got["CMD"], "ESC/POS,STAR")

	// Repeated identical value should not be duplicated
	rawSame := "CMD:ESC/POS;COMMAND:ESC/POS;"
	gotSame := parseDeviceID(rawSame)
	testutil.ExpectedEqual(t, gotSame["CMD"], "ESC/POS")
}

func TestParseDeviceID_MalformedAndEdgeCases(t *testing.T) {
	// Empty segments, missing colon, trailing semicolons
	raw := ";;;NO_COLON;EMPTY_VAL:;:EMPTY_KEY;  ;MFG:ACME;"
	got := parseDeviceID(raw)

	testutil.ExpectedLen(t, mapKeys(got), 1)
	testutil.ExpectedEqual(t, got["MFG"], "ACME")
}

func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
