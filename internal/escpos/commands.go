package escpos

import (
	"encoding/base64"
	"fmt"
)

const (
	ESC byte = 0x1B
	GS  byte = 0x1D
	LF  byte = 0x0A

	maxImgSliceHeight = 255
)

var (
	// CmdCut full paper cut.
	CmdCut = []byte{LF, GS, 0x56, 0x41, LF}

	CmdPulse = []byte{
		ESC, 0x3D, 0x01,
		ESC, 0x70, 0x00, 0x19, 0x19, // pin 2
		ESC, 0x70, 0x01, 0x19, 0x19, // pin 5
	}

	// CmdInit Resets the printer to its default state.
	CmdInit = []byte{ESC, 0x40}
)

type TextAttrs struct {
	Align        string // "left" | "center" | "right"
	Font         string // "font_a" | "font_b"
	Em           *bool
	Underline    *bool
	DoubleWidth  *bool
	DoubleHeight *bool
}

func BuildText(text string, a TextAttrs) []byte {
	var b []byte

	// --- Alignment ---
	switch a.Align {
	case "center":
		b = append(b, ESC, 0x61, 0x01)

	case "right":
		b = append(b, ESC, 0x61, 0x02)

	case "left":
		b = append(b, ESC, 0x61, 0x00)

	}

	// --- Character size (GS ! n) ---
	dw := a.DoubleWidth != nil && *a.DoubleWidth
	dh := a.DoubleHeight != nil && *a.DoubleHeight
	if dw || dh {
		var sizeFlag byte
		if dw {
			sizeFlag |= 0x10
		}
		if dh {
			sizeFlag |= 0x01
		}
		b = append(b, GS, 0x21, sizeFlag)
	}

	// --- Font (ESC M n) ---
	switch a.Font {

	case "font_a":
		b = append(b, ESC, 0x4D, 0x00)
	case "font_b":
		b = append(b, ESC, 0x4D, 0x01)
	case "font_c":
		b = append(b, ESC, 0x4D, 0x02)

	}

	// --- Em (ESC E n) ---
	if a.Em != nil {
		if *a.Em {
			b = append(b, ESC, 0x45, 0x01)
		} else {
			b = append(b, ESC, 0x45, 0x00)
		}
	}

	// --- Underline (ESC - n) ---
	if a.Underline != nil {
		if *a.Underline {
			b = append(b, ESC, 0x2D, 0x01)
		} else {
			b = append(b, ESC, 0x2D, 0x00)
		}
	}

	return append(b, []byte(text)...)

}

type ImageAttrs struct {
	Align  string
	Width  int
	Height int
}

// BuildImage decodes a base64-encoded 1-bit bitmap and returns the ESC/POS
// GS v 0 command stream needed to print it. The image is sliced into chunks
// of at most 255 rows to satisfy the command's yL byte limit.
func BuildImage(b64data string, a ImageAttrs) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(b64data)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
	}

	bytesPerRow := (a.Width + 7) / 8
	expectedLen := bytesPerRow * a.Height

	switch {
	case len(raw) < expectedLen:
		return nil, fmt.Errorf("image data too short: expected %d bytes, got %d", expectedLen, len(raw))
	case len(raw) > expectedLen:
		raw = raw[:expectedLen]
	}

	var b []byte

	// Alignment prefix.
	switch a.Align {
	case "center":
		b = append(b, ESC, 0x61, 0x01)

	case "right":
		b = append(b, ESC, 0x61, 0x02)

	}

	// GS v 0 (normal density): GS 0x76 0x30 0x00 xL xH yL yH <data>
	xL := byte(bytesPerRow & 0xFF)
	xH := byte((bytesPerRow >> 8) & 0xFF)

	for top := 0; top < a.Height; top += maxImgSliceHeight {
		h := min(a.Height-top, maxImgSliceHeight)
		yL := byte(h & 0xFF)
		yH := byte((h >> 8) & 0xFF)
		start := top * bytesPerRow
		b = append(b, GS, 0x76, 0x30, 0x00, xL, xH, yL, yH)
		b = append(b, raw[start:start+h*bytesPerRow]...)
	}

	return b, nil
}
