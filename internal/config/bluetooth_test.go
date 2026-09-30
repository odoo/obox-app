package config

import (
	"path/filepath"
	"sync"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestManager_BluetoothPrinters(t *testing.T) {
	tempDir := t.TempDir()

	cm := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: AppConfig{},
	}

	initialList := cm.GetBluetoothPrinters()
	testutil.ExpectedNotNil(t, initialList)
	testutil.ExpectedLen(t, initialList, 0)

	// Add printer 1
	err := cm.AddBluetoothPrinter("AA:BB:CC:DD:EE:01", "BT Printer 1", "ESCPOS", 0)
	testutil.ExpectedNoError(t, err)

	// Add printer 2
	err = cm.AddBluetoothPrinter("AA:BB:CC:DD:EE:02", "BT Printer 2", "ESCPOS", 0)
	testutil.ExpectedNoError(t, err)

	// Update printer 1 name (same address)
	err = cm.AddBluetoothPrinter("AA:BB:CC:DD:EE:01", "Renamed Printer 1", "ESCPOS", 0)
	testutil.ExpectedNoError(t, err)

	printers := cm.GetBluetoothPrinters()
	testutil.ExpectedLen(t, printers, 2)
	testutil.ExpectedEqual(t, printers[0].Address, "AA:BB:CC:DD:EE:01")
	testutil.ExpectedEqual(t, printers[0].Name, "Renamed Printer 1")
	testutil.ExpectedEqual(t, printers[1].Address, "AA:BB:CC:DD:EE:02")
	testutil.ExpectedEqual(t, printers[1].Name, "BT Printer 2")

	// Defensive copy test
	printers[0].Name = "MUTATED"
	testutil.ExpectedEqual(t, cm.GetBluetoothPrinters()[0].Name, "Renamed Printer 1")

	// Remove printer 1
	err = cm.RemoveBluetoothPrinter("AA:BB:CC:DD:EE:01")
	testutil.ExpectedNoError(t, err)

	afterRemove := cm.GetBluetoothPrinters()
	testutil.ExpectedLen(t, afterRemove, 1)
	testutil.ExpectedEqual(t, afterRemove[0].Address, "AA:BB:CC:DD:EE:02")

	// Remove non-existent printer -> should return nil
	err = cm.RemoveBluetoothPrinter("AA:BB:CC:DD:EE:99")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, cm.GetBluetoothPrinters(), 1)
}

func TestManager_BluetoothPrinters_Concurrent(t *testing.T) {
	tempDir := t.TempDir()

	cm := &Manager{
		path: filepath.Join(tempDir, "config.json"),
		Data: AppConfig{},
	}

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			addr := "AA:BB:CC:DD:EE:0" + string(rune('0'+id))
			_ = cm.AddBluetoothPrinter(addr, "BT Printer", "ESCPOS", 0)
			_ = cm.GetBluetoothPrinters()
			_ = cm.RemoveBluetoothPrinter(addr)
			_ = cm.GetBluetoothPrinters()
		}(i)
	}
	wg.Wait()
}
