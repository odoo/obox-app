package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestNewManager(t *testing.T) {
	cm, err := NewManager()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, cm)
	testutil.ExpectedTrue(t, cm.Path() != "", "Expected non-empty config path")
	testutil.ExpectedEqual(t, cm.Data.Port, 0)
}

func TestManager_Load(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")

	cm := &Manager{path: configFile, Data: defaults()}

	// Non-existent file.
	err := cm.Load()
	testutil.ExpectedNoError(t, err)

	// Valid config file.
	initialData := AppConfig{
		Port:        4550,
		LANPrinters: []string{"192.168.1.10", "192.168.1.20"},
	}

	raw, err := json.Marshal(initialData)
	testutil.ExpectedNoError(t, err)

	err = os.WriteFile(configFile, raw, 0644)
	testutil.ExpectedNoError(t, err)

	err = cm.Load()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, cm.Data.Port, 4550)
	testutil.ExpectedLen(t, cm.Data.LANPrinters, 2)
	testutil.ExpectedEqual(t, cm.Data.LANPrinters[0], "192.168.1.10")
	testutil.ExpectedEqual(t, cm.Data.LANPrinters[1], "192.168.1.20")

	// Corrupt JSON.
	err = os.WriteFile(configFile, []byte("{corrupt-json"), 0644)
	testutil.ExpectedNoError(t, err)

	err = cm.Load()
	testutil.ExpectedError(t, err)
}

func TestManager_Save(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")

	cm := &Manager{
		path: configFile,
		Data: AppConfig{
			Port:        4548,
			LANPrinters: []string{"10.0.0.5"},
		},
	}

	// Save successfully.
	err := cm.Save()
	testutil.ExpectedNoError(t, err)

	// Verify saved content.
	data, err := os.ReadFile(configFile)
	testutil.ExpectedNoError(t, err)

	var loaded AppConfig
	err = json.Unmarshal(data, &loaded)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, loaded.Port, 4548)
	testutil.ExpectedLen(t, loaded.LANPrinters, 1)
	testutil.ExpectedEqual(t, loaded.LANPrinters[0], "10.0.0.5")

	// Save to an invalid path.
	cm.path = tempDir

	err = cm.Save()
	testutil.ExpectedError(t, err)
}

func TestIsPortAvailable(t *testing.T) {
	port := testutil.GetFreePort(t)
	testutil.ExpectedTrue(t, isPortAvailable(port), "Expected port to be available")

	// Occupy the port and test again
	lnOccupied, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	testutil.ExpectedNoError(t, err)
	defer lnOccupied.Close()

	testutil.ExpectedFalse(t, isPortAvailable(port), "Expected port to be occupied")
}

func TestFindAvailablePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	testutil.ExpectedNoError(t, err)
	defer ln.Close()

	port, err := findAvailablePort(PortRangeStart, PortRangeEnd)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, port >= PortRangeStart && port <= PortRangeEnd, "Expected port in range")
}

func TestManager_ResolvePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	testutil.ExpectedNoError(t, err)
	defer ln.Close()

	tempDir := t.TempDir()

	// Case 1: Port is 0 -> Should find available port in range and save
	cm := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: AppConfig{Port: 0},
	}

	resolved, err := cm.ResolvePort()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, resolved >= PortRangeStart && resolved <= PortRangeEnd)
	testutil.ExpectedEqual(t, cm.Data.Port, resolved)

	// Case 2: Port is already set and available -> Should return existing port
	resolved2, err := cm.ResolvePort()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resolved2, resolved)

	// Case 3: Port is set but occupied -> Should resolve a new port
	ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", resolved))
	testutil.ExpectedNoError(t, err)
	defer ln.Close()

	resolved3, err := cm.ResolvePort()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotEqual(t, resolved3, resolved)
}

func TestManager_LANPrinters(t *testing.T) {
	tempDir := t.TempDir()

	cm := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: AppConfig{},
	}

	initialList := cm.GetLANPrinters()
	testutil.ExpectedNotNil(t, initialList)
	testutil.ExpectedLen(t, initialList, 0)

	// Add printer 1
	err := cm.AddLanEposPrinter("192.168.1.100")
	testutil.ExpectedNoError(t, err)

	// Add printer 2
	err = cm.AddLanEposPrinter("192.168.1.101")
	testutil.ExpectedNoError(t, err)

	// Add duplicate -> should be a no-op
	err = cm.AddLanEposPrinter("192.168.1.100")
	testutil.ExpectedNoError(t, err)

	printers := cm.GetLANPrinters()
	testutil.ExpectedLen(t, printers, 2)
	testutil.ExpectedEqual(t, printers[0], "192.168.1.100")
	testutil.ExpectedEqual(t, printers[1], "192.168.1.101")

	// Test defensive copy
	printers[0] = "MUTATED"
	testutil.ExpectedEqual(t, cm.GetLANPrinters()[0], "192.168.1.100")

	// Remove printer
	err = cm.RemoveLANPrinter("192.168.1.100")
	testutil.ExpectedNoError(t, err)

	afterRemove := cm.GetLANPrinters()
	testutil.ExpectedLen(t, afterRemove, 1)
	testutil.ExpectedEqual(t, afterRemove[0], "192.168.1.101")

	// Remove non-existent printer -> should return nil
	err = cm.RemoveLANPrinter("10.0.0.99")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, cm.GetLANPrinters(), 1)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()

	cm := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: AppConfig{Port: 4545},
	}

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ip := fmt.Sprintf("192.168.1.%d", workerID)
			_ = cm.AddLanEposPrinter(ip)
			_ = cm.GetLANPrinters()
			_ = cm.RemoveLANPrinter(ip)
			_ = cm.GetLANPrinters()
		}(i)
	}

	wg.Wait()
}

func TestFindAvailablePort_RangeExhausted(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	testutil.ExpectedNoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	defer ln.Close()

	result, err := findAvailablePort(port, port)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrNoAvailablePort))
	testutil.ExpectedEqual(t, result, 0)
}
