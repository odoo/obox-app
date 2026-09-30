package bluetooth

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"epos-proxy/internal/logger"
)

// btConnectTimeout is the maximum time allowed for a single RFCOMM connect attempt.
const btConnectTimeout = 3 * time.Second

type BluetoothPrinterInfo struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Device  string `json:"device"`
}

type DependencyStatus struct {
	Name        string `json:"name"`
	InstallCmd  string `json:"installCmd"`
	Description string `json:"description"`
}

// bleTransportSingleton is the shared singleton instance of bleTransport.
var bleTransportSingleton = &bleTransport{}

func supportedTransportsByOS() []transport {
	if runtime.GOOS == "darwin" {
		return []transport{
			bleTransportSingleton,
		}
	}

	return []transport{
		&classicTransport{},
	}
}

// Dial attempts to connect to the Bluetooth device at address using the platform's
// preferred transports in order, automatically falling back if a connection fails.
func Dial(address string) (net.Conn, error) {
	address, err := ValidateAddress(address)
	if err != nil {
		return nil, err
	}

	var lastErr error
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, t := range supportedTransportsByOS() {
		if !t.isAvailable() {
			logger.Debugf("BT/adapter: transport %s is not available, skipping", t.name())
			continue
		}

		logger.Debugf("BT/adapter: attempting connection via %s to %s", t.name(), address)
		conn, err := t.dial(ctx, address)
		if err == nil {
			logger.Debugf("BT/adapter: connected via %s to %s", t.name(), address)
			return conn, nil
		}
		logger.Warnf("BT/adapter: connection via %s to %s failed: %v", t.name(), address, err)
		lastErr = err
	}

	if lastErr == nil {
		return nil, fmt.Errorf("bluetooth/adapter: no available Bluetooth transports")
	}
	return nil, fmt.Errorf("bluetooth/adapter: all connection strategies failed: %w", lastErr)
}

func Scan() ([]BluetoothPrinterInfo, error) {
	if !IsActive() {
		return nil, fmt.Errorf("Please turn on Bluetooth adapter and try again.")
	}
	logger.Debug("BT/adapter: starting Bluetooth printer scan across all available transports")

	var allDevices []BluetoothPrinterInfo
	seen := make(map[string]bool)
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, t := range supportedTransportsByOS() {
		if !t.isAvailable() {
			continue
		}
		wg.Add(1)
		go func(trans transport) {
			defer wg.Done()
			devices, err := trans.scan(ctx)
			if err != nil {
				logger.Warnf("BT/adapter: transport %s scan failed: %v", trans.name(), err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, d := range devices {
				norm, err := ValidateAddress(d.Address)
				if err != nil {
					logger.Warnf("BT/adapter: skipping device with invalid address %q: %v", d.Address, err)
					continue
				}
				if !seen[norm] {
					seen[norm] = true
					d.Address = norm
					allDevices = append(allDevices, d)
				}
			}
		}(t)
	}

	wg.Wait()

	sort.SliceStable(allDevices, func(i, j int) bool {
		if allDevices[i].Device != allDevices[j].Device {
			return strings.ToLower(allDevices[i].Device) > strings.ToLower(allDevices[j].Device)
		}
		return strings.ToLower(allDevices[i].Name) < strings.ToLower(allDevices[j].Name)
	})

	logger.Debugf("BT/adapter: scan complete, found %d unique device(s)", len(allDevices))
	return allDevices, nil
}

func IsActive() bool {
	for _, t := range supportedTransportsByOS() {
		if t.isAvailable() {
			return true
		}
	}
	return false
}

func CheckDependencies() []DependencyStatus {
	return checkDependencies()
}
