package printer

import (
	"obox-app/internal/logger"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/gousb"
)

// DeviceID holds the normalised IEEE-1284 device ID keys (MFG, MDL, CMD, CLS)
// reported by a printer.
type DeviceID map[string]string

var nonAlphaRegex = regexp.MustCompile(`[^A-Z]+`)
var keyAliases = map[string]string{
	"CMD":         "CMD",
	"COMMAND SET": "CMD",
	"COMMANDSET":  "CMD",
	"COMMAND":     "CMD",
	"COMMANDS":    "CMD",

	"MFG":          "MFG",
	"MANUFACTURER": "MFG",

	"MDL":   "MDL",
	"MODEL": "MDL",

	"CLS":   "CLS",
	"CLASS": "CLS",
}

func getPrinterDeviceID(dev *gousb.Device) DeviceID {
	buf := make([]byte, 1024)

	for _, cfg := range dev.Desc.Configs {
		for _, iFace := range cfg.Interfaces {
			for _, alt := range iFace.AltSettings {
				if alt.Class != gousb.ClassPrinter && !isKnownPrinter(dev.Desc) {
					continue
				}
				n, err := dev.Control(
					0xA1,
					0x00,
					0x00,
					uint16(iFace.Number),
					buf,
				)
				if err != nil || n < 2 {
					continue
				}

				totalLen := int(buf[0])<<8 | int(buf[1])
				if totalLen <= 2 {
					continue
				}

				strLen := min(totalLen-2, n-2)
				if strLen <= 0 {
					continue
				}

				raw := sanitizeDeviceID(string(buf[2 : 2+strLen]))
				deviceID := parseDeviceID(raw)
				deviceID["RAW"] = raw
				return deviceID
			}
		}
	}
	logger.Warnf("device id not found for device: VID=%s, PID=%s", dev.Desc.Vendor, dev.Desc.Product)
	return DeviceID{}
}

func parseDeviceID(raw string) DeviceID {
	result := make(DeviceID)

	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}

		key := normalizeKey(kv[0])
		val := strings.TrimSpace(kv[1])

		if key == "" || val == "" {
			continue
		}

		if existing, ok := result[key]; ok {
			if !strings.Contains(existing, val) {
				result[key] = existing + "," + val
			}
			continue
		}

		result[key] = val
	}

	return result
}

func normalizeKey(key string) string {
	key = strings.ToUpper(strings.TrimSpace(key))

	if alias, ok := keyAliases[key]; ok {
		return alias
	}

	return key
}

func sanitizeDeviceID(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")

	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
}
