//go:build darwin

package bluetooth

import (
	"context"
	"errors"
	"net"
)

type classicTransport struct{}

func (t *classicTransport) name() string {
	return "Classic"
}

func (t *classicTransport) isAvailable() bool {
	return false
}

func (t *classicTransport) dial(ctx context.Context, address string) (net.Conn, error) {
	return nil, errors.New("not implemented for darwin")
}

func (t *classicTransport) scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return nil, errors.New("not implemented for darwin")
}

func checkDependencies() []DependencyStatus {
	return []DependencyStatus{}
}
