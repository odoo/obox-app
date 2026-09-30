//go:build darwin && cgo

package bluetooth

import (
	"context"
	"encoding/json"
	"epos-proxy/internal/logger"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tinygoBT "tinygo.org/x/bluetooth"
)

// discoverPrinterCharacteristic picks the first characteristic that accepts a
// write by probing each one with a zero-byte attempt.
func discoverPrinterCharacteristic(service tinygoBT.DeviceService) (*tinygoBT.DeviceCharacteristic, error) {
	chars, err := service.DiscoverCharacteristics(nil)
	if err != nil {
		return nil, err
	}
	if len(chars) == 0 {
		return nil, fmt.Errorf("BT/ble: service %s exposes no characteristics", service.UUID())
	}

	for i := range chars {
		c := &chars[i]
		// Probe writability using a single zero-byte payload. CoreBluetooth performs
		// a local property-flag check before sending anything to the peripheral, so
		// this reliably detects Write/WriteWithoutResponse support without sending
		// real ESC/POS data. We use 0x00 rather than an empty slice because
		// zero-length ATT Write Requests are technically invalid per the BLE spec and
		// some strict GATT stacks reject them even on writable characteristics.
		_, errW := c.Write([]byte{0x00})
		_, errWNR := c.WriteWithoutResponse([]byte{0x00})
		if errW == nil || errWNR == nil {
			logger.Debugf("BT/ble: selected writable characteristic %s (write_err=%v, write_nr_err=%v)", c.UUID(), errW, errWNR)
			return c, nil
		}
		logger.Debugf("BT/ble: characteristic %s not writable (write_err=%v, write_nr_err=%v)", c.UUID(), errW, errWNR)
	}

	logger.Errorf("BT/ble: service %s has no writable characteristics", service.UUID())
	return nil, fmt.Errorf("BT/ble: service %s has no writable characteristics", service.UUID())
}

// resolveMACToBLEUUID resolves a Classic MAC address to a BLE UUID on macOS by
// querying system_profiler for the device's Bluetooth name, then scanning for a
// BLE device with a matching or similar name.
//
// NOTE (Known Limitation):
// Matching Bluetooth device names via substring after stripping non-alphanumerics is a best-effort
// heuristic required because macOS CoreBluetooth obscures peripheral MAC addresses behind generated UUIDs.
// This heuristic can produce false positives if multiple paired devices share similar prefix names
// (e.g. "EPSON-58" vs "EPSON-580"). To mitigate this, exact sanitized matches are evaluated first before
// falling back to substring matching.
func resolveMACToBLEUUID(mac string) (string, bool) {
	btName := lookupBluetoothName(mac)
	if btName == "" {
		return "", false
	}

	sanitizedTarget := sanitizeForCUName(btName)
	if sanitizedTarget == "" {
		return "", false
	}

	logger.Debugf("BT/darwin/classic: attempting to resolve MAC %s (%q) via BLE scan name-matching", mac, btName)
	ble := bleTransportSingleton
	if !ble.isAvailable() {
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	devices, err := ble.scan(ctx)
	if err != nil {
		logger.Errorf("BT/ble: failed to scan for BLE devices: %v", err)
		return "", false
	}

	// First pass: prefer exact sanitized name match.
	for _, dev := range devices {
		if strings.EqualFold(sanitizeForCUName(dev.Name), sanitizedTarget) {
			logger.Debugf("BT/darwin/classic: exact matched BLE device %s (%q) for classic printer %s", dev.Address, dev.Name, mac)
			return dev.Address, true
		}
	}

	// Second pass: fallback to substring match if no exact match is found.
	for _, dev := range devices {
		if strings.Contains(strings.ToLower(sanitizeForCUName(dev.Name)), sanitizedTarget) {
			logger.Debugf("BT/darwin/classic: substring matched BLE device %s (%q) for classic printer %s", dev.Address, dev.Name, mac)
			return dev.Address, true
		}
	}

	return "", false
}

func lookupBluetoothName(mac string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "system_profiler", "SPBluetoothDataType", "-json").Output()
	if ctx.Err() == context.DeadlineExceeded {
		logger.Warnf("BT/darwin: system_profiler timed out resolving BT name for %s", mac)
		return ""
	}
	if err != nil {
		logger.Warnf("BT/darwin: system_profiler failed, cannot resolve BT name for %s: %v", mac, err)
		return ""
	}

	var generic map[string]any
	if err := json.Unmarshal(out, &generic); err != nil {
		logger.Warnf("BT/darwin: failed to parse system_profiler JSON: %v", err)
		return ""
	}

	target := strings.ToLower(mac)
	var found string
	var walk func(v any)
	walk = func(v any) {
		if found != "" {
			return
		}
		switch t := v.(type) {
		case map[string]any:
			if addr, ok := t["device_address"].(string); ok && strings.ToLower(addr) == target {
				if name, ok := t["_name"].(string); ok {
					found = name
					return
				}
			}
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(generic)

	if found == "" {
		logger.Warnf("BT/darwin: no system_profiler entry matched MAC %s", mac)
	} else {
		logger.Debugf("BT/darwin: resolved MAC %s -> Bluetooth name %q", mac, found)
	}
	return found
}

func sanitizeForCUName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}
