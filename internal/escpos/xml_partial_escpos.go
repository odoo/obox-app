package escpos

import (
	"encoding/base64"
	"image"
	"image/color"
	imagedraw "image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	DefaultReceiptWidth = 576
	MaxImageWidth       = 576
)

func ParseXMLToRasterImage(body []byte, bottomPadding int) ([]byte, error) {
	ep, err := ExtractEPOSPrint(body)
	if err != nil {
		return nil, err
	}

	if HasPulseOnly(ep.Items) {
		return CmdPulse, nil
	}

	img, err := renderReceipt(ep.Items, bottomPadding)
	if err != nil {
		return nil, err
	}
	return imageToEscPosBytes(img)
}

func HasPulseOnly(items []xmlRawItem) bool {
	return len(items) == 1 && items[0].tag() == "pulse"
}

func alignOffset(align string, totalWidth, elemWidth, padding int) int {
	switch align {
	case "center":
		return (totalWidth - elemWidth) / 2
	case "right":
		return totalWidth - elemWidth - padding
	default:
		return padding
	}
}

func renderReceipt(items []xmlRawItem, bottomPadding int) (image.Image, error) {
	width := getReceiptWidth(items)
	if width <= 0 || width > MaxImageWidth {
		width = DefaultReceiptWidth
	}

	const padding = 10

	canvas := image.NewRGBA(image.Rect(0, 0, width, 8000))

	imagedraw.Draw(
		canvas,
		canvas.Bounds(),
		&image.Uniform{color.White},
		image.Point{},
		imagedraw.Src,
	)

	tt, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24, // increase here
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}

	d := &font.Drawer{Dst: canvas, Src: image.Black, Face: face}

	y := 30
	for _, item := range items {
		tag := item.tag()
		attrs := item.attrs()

		switch tag {

		case "text":
			text := cleanText(item.Content)
			charLimit := width / 18

			if charLimit < 10 {
				charLimit = 10
			}

			lines := wrapText(text, charLimit)
			for _, line := range lines {
				w := d.MeasureString(line).Round()
				x := alignOffset(attrs["align"], width, w, padding)

				d.Dot = fixed.P(x, y)
				d.DrawString(line)

				y += 40
			}

		case "feed":
			lines := parseInt(attrs["line"], 1)
			y += lines * 24

		case "image":
			raw := cleanBase64(item.Content)
			data, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				continue
			}

			w := parseInt(attrs["width"], width)
			if w <= 0 || w > MaxImageWidth {
				w = width
			}

			maxH := (len(data) * 8) / w
			h := parseInt(attrs["height"], 0)
			if h <= 0 || h > maxH {
				h = maxH
			}

			if h <= 0 {
				continue
			}

			bw := image.NewGray(image.Rect(0, 0, w, h))
			byteIndex := 0
			for yy := 0; yy < h; yy++ {
				for xx := 0; xx < w; xx += 8 {
					if byteIndex >= len(data) {
						break
					}

					b := data[byteIndex]
					byteIndex++

					for bit := 0; bit < 8; bit++ {
						px := xx + bit
						if px >= w {
							break
						}

						mask := byte(1 << (7 - bit))
						if b&mask != 0 {
							bw.SetGray(px, yy, color.Gray{Y: 0})
						} else {
							bw.SetGray(px, yy, color.Gray{Y: 255})
						}
					}
				}
			}

			x := alignOffset(attrs["align"], width, w, padding)

			imagedraw.Draw(
				canvas,
				image.Rect(x, y, x+w, y+h),
				bw,
				image.Point{},
				imagedraw.Over,
			)

			y += h + 20
		}
	}

	finalImg := canvas.SubImage(image.Rect(0, 0, width, y+bottomPadding))

	return finalImg, nil
}

func imageToEscPosBytes(img image.Image) ([]byte, error) {
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()

	bw := image.NewGray(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, bCol, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			gray := uint8((r + g + bCol) / 3 >> 8)
			if gray > 127 {
				bw.SetGray(x, y, color.Gray{Y: 255})
			} else {
				bw.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	widthBytes := (width + 7) / 8
	payload := make([]byte, 0)

	// GS v 0
	payload = append(payload, 0x1d, 0x76, 0x30, 0x00)

	// little endian width/height
	payload = append(payload,
		byte(widthBytes&0xFF),
		byte((widthBytes>>8)&0xFF),
		byte(height&0xFF),
		byte((height>>8)&0xFF),
	)

	for y := 0; y < height; y++ {
		for xByte := 0; xByte < widthBytes; xByte++ {
			var bVal byte
			for bit := 0; bit < 8; bit++ {
				x := xByte*8 + bit
				if x >= width {
					continue
				}

				if bw.GrayAt(x, y).Y == 0 {
					bVal |= (1 << (7 - bit))
				}
			}
			payload = append(payload, bVal)
		}
	}

	// Full cut
	payload = append(payload, 0x1d, 0x56, 0x00) // GS V 0

	return payload, nil
}

func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func wrapText(text string, limit int) []string {
	var lines []string

	for len(text) > limit {
		lines = append(lines, text[:limit])
		text = text[limit:]
	}

	if len(text) > 0 {
		lines = append(lines, text)
	}

	return lines
}

func getReceiptWidth(items []xmlRawItem) int {
	for _, item := range items {
		if item.tag() != "image" {
			continue
		}

		imgWidth := parseInt(item.attrs()["width"], 0)
		if imgWidth > 0 && imgWidth <= MaxImageWidth {
			return imgWidth
		}
	}

	return 0
}
