package bluetooth

import (
	"context"
	"net"
)

// transport defines the interface that all Bluetooth connection types (Classic/RFCOMM, BLE) must implement.
type transport interface {
	// name returns the display name of the transport.
	name() string

	// dial opens a connection to the given address.
	dial(ctx context.Context, address string) (net.Conn, error)

	// scan performs a scan for devices matching this transport.
	scan(ctx context.Context) ([]BluetoothPrinterInfo, error)

	// isAvailable returns true if the adapter for this transport is active and available.
	isAvailable() bool
}
