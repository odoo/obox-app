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

	"obox-app/internal/testutil"
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

func TestWebViewConfig(t *testing.T) {
	tempDir := t.TempDir()
	cm := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: defaults(),
	}

	testutil.ExpectedEqual(t, cm.GetWebViewEnabled(), false)
	testutil.ExpectedEqual(t, cm.GetWebViewURL(), "")
	testutil.ExpectedEqual(t, cm.HasWebViewPIN(), true) // default is "0000"
	testutil.ExpectedTrue(t, cm.CheckWebViewPIN("0000"))
	testutil.ExpectedFalse(t, cm.CheckWebViewPIN("1234"))
	testutil.ExpectedEqual(t, cm.GetWebViewFullscreen(), false) // default is false

	// Test Fullscreen toggle
	err := cm.SetWebViewFullscreen(true)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, cm.GetWebViewFullscreen(), true)
	err = cm.SetWebViewFullscreen(false)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, cm.GetWebViewFullscreen(), false)

	// Test Invalid URLs
	err = cm.SetWebViewURL("not_a_url")
	testutil.ExpectedError(t, err)

	err = cm.SetWebViewURL("ftp://example.com")
	testutil.ExpectedError(t, err)

	err = cm.SetWebViewURL("http://")
	testutil.ExpectedError(t, err)

	// Test Valid URL
	err = cm.SetWebViewURL("https://123481892-master-all.runbot313.odoo.com/pos-self/204?access_token=4fcea930b2a1479a")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, cm.GetWebViewURL(), "https://123481892-master-all.runbot313.odoo.com/pos-self/204?access_token=4fcea930b2a1479a")

	err = cm.SetWebViewURL("https://example.com/pos")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, cm.GetWebViewURL(), "https://example.com/pos")

	// Test Enabled
	err = cm.SetWebViewEnabled(true)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, cm.GetWebViewEnabled(), true)

	// Test PIN validation
	err = cm.SetWebViewPIN("123") // too short
	testutil.ExpectedError(t, err)

	err = cm.SetWebViewPIN("12345") // too long
	testutil.ExpectedError(t, err)

	err = cm.SetWebViewPIN("12a4") // non-digit
	testutil.ExpectedError(t, err)

	err = cm.SetWebViewPIN("9876") // valid
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, cm.CheckWebViewPIN("9876"))
	testutil.ExpectedFalse(t, cm.CheckWebViewPIN("0000"))
}

func TestWebViewExitCorner(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")
	cm := &Manager{path: configFile, Data: defaults()}

	// 1. Default should be top-right
	testutil.ExpectedEqual(t, cm.GetWebViewExitCorner(), "top-right")
	testutil.ExpectedLen(t, cm.GetWebViewExitCorners(), 1)
	testutil.ExpectedEqual(t, cm.GetWebViewExitCorners()[0], "top-right")

	// 2. Set multiple valid corners
	err := cm.SetWebViewExitCorners([]string{"top-left", "top-right", "bottom-right"})
	testutil.ExpectedNoError(t, err)
	corners := cm.GetWebViewExitCorners()
	testutil.ExpectedEqual(t, len(corners), 3)
	testutil.ExpectedTrue(t, corners[0] == "top-left" && corners[1] == "top-right" && corners[2] == "bottom-right")

	// 3. Set all 4 corners
	err = cm.SetWebViewExitCorners([]string{"top-left", "top-right", "bottom-left", "bottom-right"})
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, len(cm.GetWebViewExitCorners()), 4)

	// 4. Set invalid corner
	err = cm.SetWebViewExitCorners([]string{"middle-center"})
	testutil.ExpectedError(t, err)

	// 5. Empty corners should fail
	err = cm.SetWebViewExitCorners([]string{})
	testutil.ExpectedError(t, err)
}

func TestWebViewFullscreen_DefaultFromEmptyJSON(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")
	cm := &Manager{path: configFile, Data: defaults()}
	err := os.WriteFile(configFile, []byte(`{"webview_url":"https://example.com"}`), 0644)
	testutil.ExpectedNoError(t, err)
	err = cm.Load()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedFalse(t, cm.GetWebViewFullscreen())
}
