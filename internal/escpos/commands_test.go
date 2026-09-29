package escpos

import (
	"encoding/base64"
	"testing"

	"obox-app/internal/testutil"
)

func boolRef(v bool) *bool {
	return &v
}

func TestBuildText_Alignments(t *testing.T) {
	tests := []struct {
		name     string
		align    string
		expected []byte
	}{
		{
			name:     "align left",
			align:    "left",
			expected: []byte{ESC, 0x61, 0x00, 'H', 'i'},
		},
		{
			name:     "align center",
			align:    "center",
			expected: []byte{ESC, 0x61, 0x01, 'H', 'i'},
		},
		{
			name:     "align right",
			align:    "right",
			expected: []byte{ESC, 0x61, 0x02, 'H', 'i'},
		},
		{
			name:     "no align",
			align:    "",
			expected: []byte{'H', 'i'},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildText("Hi", TextAttrs{Align: tc.align})
			testutil.ExpectedBytesEqual(t, got, tc.expected)
		})
	}
}

func TestBuildText_CharSize(t *testing.T) {
	tests := []struct {
		name     string
		dw       *bool
		dh       *bool
		expected []byte
	}{
		{
			name:     "double width only",
			dw:       boolRef(true),
			dh:       nil,
			expected: []byte{GS, 0x21, 0x10, 'A'},
		},
		{
			name:     "double height only",
			dw:       nil,
			dh:       boolRef(true),
			expected: []byte{GS, 0x21, 0x01, 'A'},
		},
		{
			name:     "both double width and height",
			dw:       boolRef(true),
			dh:       boolRef(true),
			expected: []byte{GS, 0x21, 0x11, 'A'},
		},
		{
			name:     "both false",
			dw:       boolRef(false),
			dh:       boolRef(false),
			expected: []byte{'A'},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildText("A", TextAttrs{DoubleWidth: tc.dw, DoubleHeight: tc.dh})
			testutil.ExpectedBytesEqual(t, got, tc.expected)
		})
	}
}

func TestBuildText_Font(t *testing.T) {
	tests := []struct {
		name     string
		font     string
		expected []byte
	}{
		{
			name:     "font a",
			font:     "font_a",
			expected: []byte{ESC, 0x4D, 0x00, 'X'},
		},
		{
			name:     "font b",
			font:     "font_b",
			expected: []byte{ESC, 0x4D, 0x01, 'X'},
		},
		{
			name:     "font c",
			font:     "font_c",
			expected: []byte{ESC, 0x4D, 0x02, 'X'},
		},
		{
			name:     "unknown font",
			font:     "font_z",
			expected: []byte{'X'},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildText("X", TextAttrs{Font: tc.font})
			testutil.ExpectedBytesEqual(t, got, tc.expected)
		})
	}
}

func TestBuildText_EmAndUnderline(t *testing.T) {
	tests := []struct {
		name     string
		attrs    TextAttrs
		expected []byte
	}{
		{
			name:     "em on",
			attrs:    TextAttrs{Em: boolRef(true)},
			expected: []byte{ESC, 0x45, 0x01, 'T'},
		},
		{
			name:     "em off",
			attrs:    TextAttrs{Em: boolRef(false)},
			expected: []byte{ESC, 0x45, 0x00, 'T'},
		},
		{
			name:     "underline on",
			attrs:    TextAttrs{Underline: boolRef(true)},
			expected: []byte{ESC, 0x2D, 0x01, 'U'},
		},
		{
			name:     "underline off",
			attrs:    TextAttrs{Underline: boolRef(false)},
			expected: []byte{ESC, 0x2D, 0x00, 'U'},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := "T"
			if tc.name[:3] == "und" {
				text = "U"
			}

			got := BuildText(text, tc.attrs)
			testutil.ExpectedBytesEqual(t, got, tc.expected)
		})
	}
}

func TestBuildText_Combined(t *testing.T) {
	attrs := TextAttrs{
		Align:        "center",
		DoubleWidth:  boolRef(true),
		DoubleHeight: boolRef(true),
		Font:         "font_b",
		Em:           boolRef(true),
		Underline:    boolRef(true),
	}

	got := BuildText("Hello", attrs)
	expected := []byte{
		ESC, 0x61, 0x01, // align center
		GS, 0x21, 0x11, // double width & height
		ESC, 0x4D, 0x01, // font b
		ESC, 0x45, 0x01, // em on
		ESC, 0x2D, 0x01, // ul on
		'H', 'e', 'l', 'l', 'o',
	}
	testutil.ExpectedBytesEqual(t, got, expected)
}

func TestBuildImage_LeftAlignment(t *testing.T) {
	bitmap := []byte{0xFF}
	b64 := base64.StdEncoding.EncodeToString(bitmap)

	got, err := BuildImage(b64, ImageAttrs{
		Align:  "left",
		Width:  8,
		Height: 1,
	})
	testutil.ExpectedNoError(t, err)

	expected := []byte{
		GS, 0x76, 0x30, 0x00,
		0x01, 0x00,
		0x01, 0x00,
		0xFF,
	}

	testutil.ExpectedBytesEqual(t, got, expected)
}

func TestBuildImage_Valid(t *testing.T) {
	bitmap := []byte{0xFF, 0x00, 0xAA, 0x55}
	b64 := base64.StdEncoding.EncodeToString(bitmap)

	attrs := ImageAttrs{
		Align:  "center",
		Width:  16,
		Height: 2,
	}

	got, err := BuildImage(b64, attrs)
	testutil.ExpectedNoError(t, err)

	expectedPrefix := []byte{
		ESC, 0x61, 0x01, // center align
		GS, 0x76, 0x30, 0x00,
		0x02, 0x00, // xL=2, xH=0
		0x02, 0x00, // yL=2, yH=0
	}
	expected := append(expectedPrefix, bitmap...)
	testutil.ExpectedBytesEqual(t, got, expected)
}

func TestBuildImage_RawBase64(t *testing.T) {
	bitmap := []byte{0x12, 0x34}
	b64 := base64.RawStdEncoding.EncodeToString(bitmap)

	attrs := ImageAttrs{
		Align:  "right",
		Width:  8,
		Height: 2,
	}

	got, err := BuildImage(b64, attrs)
	testutil.ExpectedNoError(t, err)

	expectedPrefix := []byte{
		ESC, 0x61, 0x02, // right align
		GS, 0x76, 0x30, 0x00,
		0x01, 0x00, // xL=1, xH=0
		0x02, 0x00, // yL=2, yH=0
	}
	expected := append(expectedPrefix, bitmap...)
	testutil.ExpectedBytesEqual(t, got, expected)

	_, err = BuildImage("!!invalid-base64!!", ImageAttrs{Width: 8, Height: 1})
	testutil.ExpectedError(t, err)
}

func TestBuildImage_DataHandling(t *testing.T) {
	// Data too short.
	bitmap := []byte{0x01, 0x02}
	b64 := base64.StdEncoding.EncodeToString(bitmap)

	_, err := BuildImage(b64, ImageAttrs{Width: 16, Height: 2})
	testutil.ExpectedError(t, err)

	// Data longer than expected is truncated.
	bitmap = []byte{0x01, 0x02, 0x99, 0x99}
	b64 = base64.StdEncoding.EncodeToString(bitmap)

	got, err := BuildImage(b64, ImageAttrs{Width: 8, Height: 2})
	testutil.ExpectedNoError(t, err)

	expectedPrefix := []byte{
		GS, 0x76, 0x30, 0x00,
		0x01, 0x00, // xL=1, xH=0
		0x02, 0x00, // yL=2, yH=0
		0x01, 0x02, // truncated data
	}
	testutil.ExpectedBytesEqual(t, got, expectedPrefix)

	// Image data larger than one slice is split into multiple chunks.
	bitmap = make([]byte, 300)
	for i := range bitmap {
		bitmap[i] = byte(i % 256)
	}
	b64 = base64.StdEncoding.EncodeToString(bitmap)

	got, err = BuildImage(b64, ImageAttrs{Width: 8, Height: 300})
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, got, 316)

	// Verify Chunk 1 header
	testutil.ExpectedBytesEqual(t, got[:8], []byte{GS, 0x76, 0x30, 0x00, 0x01, 0x00, 0xFF, 0x00})

	// Verify Chunk 2 header (0x2D = 45)
	testutil.ExpectedBytesEqual(t, got[263:263+8], []byte{GS, 0x76, 0x30, 0x00, 0x01, 0x00, 0x2D, 0x00})
}
