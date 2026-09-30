package bluetooth

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var UuidRegexp = regexp.MustCompile(`^(?i)[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)
var MacRegexp = regexp.MustCompile(`^(?i)([0-9A-F]{2}[:\-]){5}[0-9A-F]{2}$`)

var printerKeywords = []string{"printer", "pos", "receipt", "thermal"}

func looksLikePrinter(name string) bool {
	name = strings.ToLower(name)
	for _, kw := range printerKeywords {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// parseMACToBytes converts "AA:BB:CC:DD:EE:FF" → [6]byte in reversed order
// (little-endian as required by the BlueZ sockaddr_rc).
func parseMACToBytes(macAddress string) ([6]byte, error) {
	parts := strings.Split(strings.ToUpper(macAddress), ":")
	if len(parts) != 6 {
		return [6]byte{}, fmt.Errorf("invalid MAC: %s", macAddress)
	}
	var b [6]byte
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return [6]byte{}, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		b[5-i] = byte(v)
	}
	return b, nil
}

func ValidateAddress(address string) (string, error) {
	address = strings.ToUpper(strings.TrimSpace(address))
	if !UuidRegexp.MatchString(address) {
		address = strings.ReplaceAll(address, "-", ":")
	}

	if MacRegexp.MatchString(address) || UuidRegexp.MatchString(address) {
		return address, nil
	}
	return "", fmt.Errorf("invalid MAC address format: %s (expected format: AA:BB:CC:DD:EE:FF or UUID)", address)
}
