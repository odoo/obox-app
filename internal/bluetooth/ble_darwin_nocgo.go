//go:build darwin && !cgo

package bluetooth

import (
	"context"
	"errors"
	"net"
)

type bleTransport struct{}

func (t *bleTransport) name() string {
	return "BLE"
}

func (t *bleTransport) isAvailable() bool {
	return false
}

func (t *bleTransport) dial(ctx context.Context, address string) (net.Conn, error) {
	return nil, errors.New("BLE on macOS requires CGO_ENABLED=1")
}

func (t *bleTransport) scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return nil, errors.New("BLE on macOS requires CGO_ENABLED=1")
}
