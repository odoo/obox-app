package printer

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"epos-proxy/internal/testutil"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(0, nil)
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

	mgr := NewManager(0, nil)
	printerID := EncodeLANPrinterID("127.0.0.1")

	testPayload := []byte("TEST PRINT DATA FOR LAN")
	replyChan, err := mgr.WriteAsync(printerID, testPayload)
	testutil.ExpectedNoError(t, err)

	select {
	case res := <-replyChan:
		testutil.ExpectedTrue(t, res.OK)
		testutil.ExpectedNoError(t, res.Err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for print job reply")
	}

	select {
	case <-done:
		mu.Lock()
		testutil.ExpectedBytesEqual(t, receivedData, testPayload)
		mu.Unlock()
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for mock printer server to receive data")
	}
}

func TestManager_Get_Error_And_Reusing(t *testing.T) {
	mgr := NewManager(0, nil)

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
	mgr := NewManager(0, nil)

	// Non-existent USB printer
	nonExistentID := "czpOT05fRVhJU1RFTlRfU0VSSUFMCg"
	replyChan, err := mgr.WriteAsync(nonExistentID, []byte("data"))
	testutil.ExpectedError(t, err)
	testutil.ExpectedNil(t, replyChan)
}
