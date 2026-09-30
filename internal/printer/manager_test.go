package printer

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil)
	testutil.ExpectedNotNil(t, mgr)
	testutil.ExpectedNotNil(t, mgr.printers)
}

func TestPrinter_QueueFull(t *testing.T) {
	p := &Printer{
		connectionType: ConnKindLAN,
		lanIP:          "127.0.0.1",
		jobs:           make(chan Job, 2),
	}
	// Note: We do not start p.loop() here so we can fill the queue deterministically

	// Fill the queue to QueueSize (2)
	for range 2 {
		err := p.Enqueue(func(p *Printer) JobResult {
			return JobResult{OK: true}
		}, nil)
		testutil.ExpectedNoError(t, err)
	}

	// 3rd enqueue must return ErrQueueFull
	err := p.Enqueue(func(p *Printer) JobResult {
		return JobResult{OK: true}
	}, nil)

	testutil.ExpectedTrue(t, errors.Is(err, ErrQueueFull))
}

func TestManager_LANPrinterIntegration(t *testing.T) {
	var receivedData []byte
	var mu sync.Mutex
	done := make(chan struct{})

	// Start a mock TCP printer listener on 127.0.0.1:9100
	_, _, err := testutil.StartMockTCPServer(t, func(conn net.Conn) {
		buf := make([]byte, 1024)
		n, _ := io.ReadAtLeast(conn, buf, 1)
		mu.Lock()
		receivedData = append(receivedData, buf[:n]...)
		mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
	})
	testutil.ExpectedNoError(t, err)

	cfg := &config.Manager{
		Data: config.AppConfig{
			LANPrinters: []string{
				"127.0.0.1",
			},
		},
	}
	mgr := NewManager(cfg)
	printerID := EncodeLANPrinterID("127.0.0.1")

	testPayload := []byte("TEST PRINT DATA FOR LAN")
	replyChan, err := mgr.WriteAsync(printerID, testPayload)
	testutil.ExpectedNoError(t, err)

	select {
	case res := <-replyChan:
		testutil.ExpectedTrue(t, res.OK)
		testutil.ExpectedNoError(t, res.Err)
	case <-time.After(5 * time.Second):
		testutil.ExpectedTrue(t, false, "Timed out waiting for print job reply")
	}

	select {
	case <-done:
		mu.Lock()
		testutil.ExpectedBytesEqual(t, receivedData, testPayload)
		mu.Unlock()
	case <-time.After(3 * time.Second):
		testutil.ExpectedTrue(t, false, "Timed out waiting for mock printer server to receive data")
	}
}

func TestManager_Get_Error_And_Reusing(t *testing.T) {
	cfg := &config.Manager{
		Data: config.AppConfig{
			LANPrinters: []string{
				"127.0.0.1",
			},
		},
	}
	mgr := NewManager(cfg)

	// 1. Unreachable LAN printer returns error
	_, err := mgr.Get(EncodeLANPrinterID("127.0.0.254"))
	testutil.ExpectedError(t, err)

	// 2. Mock LAN printer on port 9100
	_, _, err = testutil.StartMockTCPServer(t)
	testutil.ExpectedNoError(t, err)

	id := EncodeLANPrinterID("127.0.0.1")
	p1, err := mgr.Get(id)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, p1)

	// Reusing same printer from manager
	p2, err := mgr.Get(id)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, p1, p2)
}

func TestManager_WriteAsync_PrinterNotFound(t *testing.T) {
	mgr := NewManager(nil)

	// Non-existent USB printer
	nonExistentID := "czpOT05fRVhJU1RFTlRfU0VSSUFMCg"
	replyChan, err := mgr.WriteAsync(nonExistentID, []byte("data"))
	testutil.ExpectedError(t, err)
	testutil.ExpectedNil(t, replyChan)
}

func TestManager_ConvertBody(t *testing.T) {
	mgr := NewManager(nil)
	xmlPayload := []byte(`<epos-print><text>Test Receipt</text></epos-print>`)

	convert := func(m *Manager, id string, payload []byte) ([]byte, error) {
		p := &Printer{}
		m.setProtocol(id, p)
		return p.ConvertBody(payload)
	}

	// 1. Standard ESCPOS printer (LAN or Epson VID:PID)
	epsonID, err := encodePrinterID(&LibUsbPrinter{VidPid: "04b8:0202", Serial: "123"})
	testutil.ExpectedNoError(t, err)
	dataStandard, err := convert(mgr, epsonID, xmlPayload)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(dataStandard) > 0)

	// 2. Partial ESCPOS printer (STMicroelectronics 0483:5720) -> raster image
	partialID, err := encodePrinterID(&LibUsbPrinter{VidPid: "0483:5720", Serial: "456"})
	testutil.ExpectedNoError(t, err)
	dataPartial, err := convert(mgr, partialID, xmlPayload)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(dataPartial) > 0)
	// Partial ESC/POS starts with GS v 0 raster command (0x1d 0x76 0x30 0x00)
	testutil.ExpectedEqual(t, dataPartial[0], byte(0x1d))
	testutil.ExpectedEqual(t, dataPartial[1], byte(0x76))
	testutil.ExpectedEqual(t, dataPartial[2], byte(0x30))
	testutil.ExpectedEqual(t, dataPartial[3], byte(0x00))

	// 3. Resolved printer converts according to its protocol and padding
	partialPrinter := &Printer{protocol: ProtocolESCPOSPartial}
	dataFromPrinter, err := partialPrinter.ConvertBody(xmlPayload)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, dataFromPrinter[0], byte(0x1d))
	testutil.ExpectedEqual(t, dataFromPrinter[1], byte(0x76))

	// 4. Invalid XML returns error
	_, err = convert(mgr, epsonID, []byte("invalid xml"))
	testutil.ExpectedError(t, err)

	// 5. Configured Bluetooth and LAN printers
	cfg := &config.Manager{
		Data: config.AppConfig{
			BluetoothPrinters: []config.BluetoothPrinterConfig{
				{Address: "11:22:33:44:55:66", Name: "BT Partial", Protocol: string(ProtocolESCPOSPartial)},
				{Address: "22:33:44:55:66:77", Name: "BT Standard", Protocol: string(ProtocolESCPOS)},
			},
			LANPrinters: []string{
				"192.168.1.200",
			},
		},
	}
	mgrWithCfg := NewManager(cfg)

	// BT Partial -> raster
	btPartialData, err := convert(mgrWithCfg, encodeBluetoothPrinterID("11:22:33:44:55:66"), xmlPayload)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, btPartialData[0], byte(0x1d))
	testutil.ExpectedEqual(t, btPartialData[1], byte(0x76))

	// BT Standard -> ESCPOS
	btStandardData, err := convert(mgrWithCfg, encodeBluetoothPrinterID("22:33:44:55:66:77"), xmlPayload)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(btStandardData) > 0)
	testutil.ExpectedNotEqual(t, btStandardData[0], byte(0x1d))

	// LAN -> ESCPOS
	lanStandardData, err := convert(mgrWithCfg, EncodeLANPrinterID("192.168.1.200"), xmlPayload)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, len(lanStandardData) > 0)
	testutil.ExpectedNotEqual(t, lanStandardData[0], byte(0x1d))
}
