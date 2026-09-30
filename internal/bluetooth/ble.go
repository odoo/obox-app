//go:build !darwin || cgo

package bluetooth

import (
	"context"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"epos-proxy/internal/logger"

	"tinygo.org/x/bluetooth"
)

const defaultBLEWriteChunk = 180 // Safe MTU chunk size is 180 bytes.

var adapter = bluetooth.DefaultAdapter
var adapterMu sync.Mutex
var adapterEnabled bool

// bleCharCache stores the last-known writable characteristic UUID per device
// address (key: normalised address string, value: characteristic UUID string).
// On reconnect, the cached UUID is matched against the newly discovered
// characteristics, skipping the probe-write entirely. This avoids sending a
// stray 0x00 byte to non-selected characteristics on every dial.
var bleCharCache sync.Map

func enableAdapter() error {
	adapterMu.Lock()
	defer adapterMu.Unlock()
	if adapterEnabled {
		return nil
	}
	if err := adapter.Enable(); err != nil {
		logger.Errorf("BT/ble: failed to enable bluetooth adapter: %v", err)
		return err
	}
	logger.Debugf("BT/ble: bluetooth adapter enabled successfully")
	adapterEnabled = true
	return nil
}

type bleTransport struct {
	mu sync.Mutex // guards adapter Scan and Connect calls
}

func (t *bleTransport) name() string {
	return "BLE"
}

func (t *bleTransport) isAvailable() bool {
	return enableAdapter() == nil
}

func (t *bleTransport) dial(ctx context.Context, address string) (net.Conn, error) {
	// macOS CoreBluetooth identifies BLE peripherals by UUID rather than exposing
	// their Bluetooth MAC address. Linux and Windows expose a Bluetooth address for
	// BLE devices, but connections are still established through BLE peripheral
	// discovery rather than directly dialing an address.

	dialAddress := address
	if !UuidRegexp.MatchString(address) && runtime.GOOS == "darwin" {
		resolved, ok := resolveMACToBLEUUID(address)
		if ok {
			logger.Debugf("BT/ble: resolved MAC %s to BLE UUID %s", address, resolved)
			dialAddress = resolved
		} else {
			return nil, fmt.Errorf("BT/ble: cannot dial MAC address %s directly on macOS without UUID resolution", address)
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return dialBLE(ctx, dialAddress)
}

func (t *bleTransport) scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return scanLiveBLEPrinters(ctx)
}

func scanLiveBLEPrinters(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	logger.Debugf("BT/ble: starting live BLE scan for %v", btConnectTimeout)

	var mu sync.Mutex
	var devices []BluetoothPrinterInfo
	seen := make(map[string]bool)
	scanDone := make(chan error, 1)

	go func() {
		err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
			name := result.LocalName()
			if name == "" {
				return
			}
			addrStr := result.Address.String()

			mu.Lock()
			defer mu.Unlock()
			if seen[addrStr] {
				return
			}

			seen[addrStr] = true
			devices = append(devices, BluetoothPrinterInfo{
				Address: addrStr,
				Name:    name,
				Device:  getDeviceType(name),
			})
		})
		scanDone <- err
	}()

	select {
	case <-ctx.Done():
		_ = adapter.StopScan()
		return nil, ctx.Err()
	case <-time.After(btConnectTimeout):
		_ = adapter.StopScan()
	}

	select {
	case err := <-scanDone:
		if err != nil {
			logger.Errorf("BT/ble: scan failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		// Known limitation: if adapter.Scan's callback loop never returns (e.g. driver
		// hang), the inner goroutine leaks here. The tinygo BLE API provides no way to
		// forcibly terminate a stuck Scan goroutine beyond StopScan(), which we already
		// called above. Under normal circumstances StopScan() causes Scan to return
		// quickly; this backstop only fires if the driver is pathologically stuck.
		logger.Warnf("BT/ble: scan goroutine did not exit within backstop")
	}

	mu.Lock()
	defer mu.Unlock()
	return devices, nil
}

type bleConn struct {
	device       bluetooth.Device
	char         *bluetooth.DeviceCharacteristic
	address      string
	writeTimeout time.Duration
	readDeadline time.Time
	connected    bool
}

// Read returns io.EOF because standard BLE ESC/POS receipt printers are write-only
// data sinks (GATT write characteristics without RX/notification subscriptions configured).
// Two-way communication (e.g. printer status queries or ACK responses) requires subscribing
// to a notification/indication characteristic, which is not currently implemented in this unidirectional transport.
func (c *bleConn) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (c *bleConn) Write(b []byte) (int, error) {
	if c.char == nil {
		return 0, fmt.Errorf("BT/ble: connection closed")
	}

	logger.Debugf("BT/ble: writing %d bytes to %s", len(b), c.address)

	// Split write into chunks because BLE has MTU (Maximum Transmission Unit) limit.
	totalWritten := 0

	for totalWritten < len(b) {
		end := totalWritten + defaultBLEWriteChunk
		if end > len(b) {
			end = len(b)
		}
		chunk := b[totalWritten:end]

		n, err := c.char.Write(chunk)
		if err != nil {
			n, err = c.char.WriteWithoutResponse(chunk)
			if err != nil {
				return totalWritten, fmt.Errorf("BT/ble: write failed: %w", err)
			}
			time.Sleep(15 * time.Millisecond)
		}
		if n <= 0 {
			return totalWritten, fmt.Errorf("BT/ble: write returned 0 bytes with no error (possible stall)")
		}
		totalWritten += n
	}

	return totalWritten, nil
}

func (c *bleConn) Close() error {
	if c.connected {
		err := c.device.Disconnect()
		c.connected = false
		c.char = nil
		return err
	}
	return nil
}

func (c *bleConn) LocalAddr() net.Addr {
	return netAddrPlaceholder{net: "ble", addr: "local-ble"}
}

func (c *bleConn) RemoteAddr() net.Addr {
	return netAddrPlaceholder{net: "ble", addr: c.address}
}

func (c *bleConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *bleConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *bleConn) SetWriteDeadline(t time.Time) error {
	if t.IsZero() {
		c.writeTimeout = 0
	} else {
		c.writeTimeout = time.Until(t)
	}
	return nil
}

// drainBLEConn waits for a connection on connCh and closes it if non-nil.
// This frees the peripheral when dialBLE has already returned due to a timeout
// or context cancellation but the underlying dial goroutine connects later.
func drainBLEConn(connCh <-chan net.Conn) {
	go func() {
		if conn := <-connCh; conn != nil {
			_ = conn.Close()
		}
	}()
}

func dialBLE(ctx context.Context, address string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := dialBLEInternal(address)
		connCh <- conn
		ch <- result{conn, err}
	}()
	select {
	case <-ctx.Done():
		drainBLEConn(connCh)
		return nil, ctx.Err()
	case r := <-ch:
		<-connCh // drain connCh so the goroutine doesn't leak
		return r.conn, r.err
	case <-time.After(15 * time.Second):
		drainBLEConn(connCh)
		return nil, fmt.Errorf("BT/ble: connect to %s timed out after 15s", address)
	}
}

func dialBLEInternal(address string) (conn net.Conn, err error) {
	logger.Debugf("BT/ble: connecting to %s", address)

	var addr bluetooth.Address
	addr.Set(address)

	device, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("BT/ble: failed to connect to %s: %w", address, err)
	}

	success := false
	defer func() {
		if !success {
			_ = device.Disconnect()
		}
	}()

	services, err := device.DiscoverServices(nil)
	if err != nil {
		return nil, fmt.Errorf("BT/ble: failed to discover services: %w", err)
	}

	var char *bluetooth.DeviceCharacteristic

	// Check cache: if we successfully identified the writable characteristic on a
	// previous connection, use it directly without probing.
	if cachedUUID, ok := bleCharCache.Load(address); ok {
		cachedStr := cachedUUID.(string)
		for _, service := range services {
			chars, discErr := service.DiscoverCharacteristics(nil)
			if discErr != nil {
				continue
			}
			for i := range chars {
				if chars[i].UUID().String() == cachedStr {
					char = &chars[i]
					logger.Debugf("BT/ble: using cached characteristic %s for %s", cachedStr, address)
					break
				}
			}
			if char != nil {
				break
			}
		}
		if char == nil {
			logger.Debugf("BT/ble: cached characteristic %s not found for %s, re-probing", cachedStr, address)
			bleCharCache.Delete(address)
		}
	}

	// No cache hit (first connect, or cache miss after firmware change): probe.
	if char == nil {
		for _, service := range services {
			char, err = discoverPrinterCharacteristic(service)
			if err != nil {
				logger.Debugf("BT/ble: skipping service %s: %v", service.UUID(), err)
				continue
			}
			if char != nil {
				break
			}
		}
		if char != nil {
			// Persist the winner so future reconnects skip probing.
			bleCharCache.Store(address, char.UUID().String())
			logger.Debugf("BT/ble: caching characteristic %s for %s", char.UUID(), address)
		}
	}

	if char == nil {
		return nil, fmt.Errorf("BT/ble: no writable characteristic found")
	}

	logger.Debugf("BT/ble: connected to %s using characteristic %s", address, char.UUID())

	success = true
	return &bleConn{
		device:       device,
		char:         char,
		address:      address,
		writeTimeout: 10 * time.Second,
		connected:    true,
	}, nil
}

func getDeviceType(name string) string {
	if looksLikePrinter(name) {
		return "printer"
	}

	return "other"
}
