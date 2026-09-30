//go:build !darwin

package bluetooth

import (
	"errors"

	tinygoBT "tinygo.org/x/bluetooth"
)

func discoverPrinterCharacteristic(service tinygoBT.DeviceService) (*tinygoBT.DeviceCharacteristic, error) {
	return nil, errors.New("Not implemented for Windows & Linux platforms")
}

func resolveMACToBLEUUID(mac string) (string, bool) {
	// On non-darwin platforms, we cannot resolve the MAC address to a BLE UUID.
	// macOS CoreBluetooth identifies BLE peripherals by UUID rather than exposing
	// their Bluetooth MAC address. Linux and Windows expose a Bluetooth address for
	// BLE devices, but connections are still established through BLE peripheral
	// discovery rather than directly dialing an address.
	return "", false
}
