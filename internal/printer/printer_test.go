package printer

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"obox-app/internal/testutil"

	"github.com/google/gousb"
)

func TestNewPrinter_LAN(t *testing.T) {
	lanIP := "192.168.1.55"
	encodedID := EncodeLANPrinterID(lanIP)

	p := newPrinter(encodedID)
	testutil.ExpectedNotNil(t, p)
	testutil.ExpectedEqual(t, p.connectionType, ConnKindLAN)
	testutil.ExpectedEqual(t, p.lanIP, lanIP)
	testutil.ExpectedNil(t, p.id)
	testutil.ExpectedNotNil(t, p.jobs)

	// Clean up by closing
	p.close()
}

func TestNewPrinter_USB(t *testing.T) {
	// Case 1: USB with serial
	printerWithSerial := &LibUsbPrinter{Serial: "SN987654"}
	encodedSerial, err := encodePrinterID(printerWithSerial)
	testutil.ExpectedNoError(t, err)

	pSerial := newPrinter(encodedSerial)
	testutil.ExpectedNotNil(t, pSerial)
	testutil.ExpectedEqual(t, pSerial.connectionType, ConnKindUSB)
	testutil.ExpectedNotNil(t, pSerial.id)
	testutil.ExpectedEqual(t, pSerial.id.Serial, "SN987654")
	pSerial.close()

	// Case 2: USB with VidPid and Path
	printerWithVidPidPath := &LibUsbPrinter{VidPid: "04B8:0202", Path: "1.2.3"}
	encodedVidPidPath, err := encodePrinterID(printerWithVidPidPath)
	testutil.ExpectedNoError(t, err)

	pVidPidPath := newPrinter(encodedVidPidPath)
	testutil.ExpectedNotNil(t, pVidPidPath)
	testutil.ExpectedEqual(t, pVidPidPath.connectionType, ConnKindUSB)
	testutil.ExpectedNotNil(t, pVidPidPath.id)
	testutil.ExpectedEqual(t, pVidPidPath.id.VidPid, "04B8:0202")
	testutil.ExpectedEqual(t, pVidPidPath.id.Path, "1.2.3")
	pVidPidPath.close()
}

func TestNewPrinter_EdgeCases(t *testing.T) {
	// Empty string ID -> USB with nil ID
	pEmpty := newPrinter("")
	testutil.ExpectedNotNil(t, pEmpty)
	testutil.ExpectedEqual(t, pEmpty.connectionType, ConnKindUSB)
	testutil.ExpectedNil(t, pEmpty.id)
	pEmpty.close()

	// Invalid base64 ID -> USB with nil ID (decodePrinterID fails safely)
	pInvalid := newPrinter("!not-valid-base64!")
	testutil.ExpectedNotNil(t, pInvalid)
	testutil.ExpectedEqual(t, pInvalid.connectionType, ConnKindUSB)
	testutil.ExpectedNil(t, pInvalid.id)
	pInvalid.close()
}

func TestPrinter_EnsureOpenLAN_SuccessAndReusing(t *testing.T) {
	_, _, err := testutil.StartMockTCPServer(t)
	testutil.ExpectedNoError(t, err)

	p := &Printer{connectionType: ConnKindLAN, lanIP: "127.0.0.1"}
	defer p.close()

	// First ensureOpen creates the connection
	err = p.ensureOpen()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, p.tcpConn)

	savedConn := p.tcpConn

	// Second ensureOpen reuses existing connection without error
	err = p.ensureOpen()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, p.tcpConn, savedConn)
}

func TestPrinter_EnsureOpenLAN_Offline(t *testing.T) {
	// Connect to an unreachable local IP or closed port
	p := &Printer{connectionType: ConnKindLAN, lanIP: "127.0.0.254"}
	defer p.close()

	err := p.ensureOpen()
	testutil.ExpectedError(t, err)
	testutil.ExpectedNil(t, p.tcpConn)
}

func TestPrinter_WriteLAN_Success(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	done := make(chan struct{})

	_, _, err := testutil.StartMockTCPServer(t, func(conn net.Conn) {
		buf := make([]byte, 1024)
		n, _ := io.ReadAtLeast(conn, buf, 1)
		mu.Lock()
		received = append(received, buf[:n]...)
		mu.Unlock()
		select {
		case <-done:
		default:
			close(done)
		}
	})
	testutil.ExpectedNoError(t, err)

	p := &Printer{connectionType: ConnKindLAN, lanIP: "127.0.0.1"}
	defer p.close()

	payload := []byte("ESC/POS SAMPLE RECEIPT DATA")
	err = p.Write(payload)
	testutil.ExpectedNoError(t, err)

	select {
	case <-done:
		mu.Lock()
		testutil.ExpectedBytesEqual(t, received, payload)
		mu.Unlock()
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for LAN write payload")
	}
}

func TestPrinter_WriteLAN_OfflineFails(t *testing.T) {
	p := &Printer{connectionType: ConnKindLAN, lanIP: "127.0.0.254"}
	defer p.close()

	err := p.Write([]byte("some data"))
	testutil.ExpectedError(t, err)
}

func TestPrinter_EnsureOpenUSB_NotFound(t *testing.T) {
	p := &Printer{connectionType: ConnKindUSB, id: &ID{Serial: "NON_EXISTENT_SERIAL_12345"}}
	defer p.close()

	err := p.ensureOpen()
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrNotFound))
}

func TestPrinter_EnsureOpenUSB_OpenDevicesError(t *testing.T) {
	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		return nil, errors.New("usb permission denied")
	})

	p := &Printer{connectionType: ConnKindUSB, id: &ID{Serial: "SN123"}}
	defer p.close()

	err := p.ensureOpen()
	testutil.ExpectedError(t, err)
}

func TestPrinter_EnsureOpenUSB_FindAny_NotFound(t *testing.T) {
	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		return nil, nil
	})

	// findAny mode
	p := &Printer{connectionType: ConnKindUSB, id: nil}
	defer p.close()

	err := p.ensureOpen()
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrNotFound))
}

func TestPrinter_EnsureOpenUSB_AlreadyOpen(t *testing.T) {
	p := &Printer{
		connectionType: ConnKindUSB,
		device:         &gousb.Device{},
	}
	// already connected, returns nil without calling openDevices
	err := p.ensureOpen()
	testutil.ExpectedNoError(t, err)
}

func TestPrinter_EnsureOpenUSB_VidPidFiltering(t *testing.T) {
	epsonDesc := testutil.MockEpsonPrinterDesc() // VID 04B8, PID 0202
	zebraDesc := testutil.MockZebraPrinterDesc() // VID 0A5F, PID 0187

	var filteredResults []bool
	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		filteredResults = append(filteredResults, fn(zebraDesc)) // should be false
		filteredResults = append(filteredResults, fn(epsonDesc)) // should be true
		return nil, nil
	})

	// Target Epson printer with VidPid: "04B8:0202"
	p := &Printer{
		connectionType: ConnKindUSB,
		id:             &ID{VidPid: "04B8:0202", Path: "1.1.2"},
	}
	defer p.close()

	// ensureOpen calls openDevices which runs the filter predicate
	err := p.ensureOpen()
	testutil.ExpectedTrue(t, errors.Is(err, ErrNotFound))

	// Verify Zebra was filtered out and Epson was accepted
	testutil.ExpectedLen(t, filteredResults, 2)
	testutil.ExpectedFalse(t, filteredResults[0]) // zebra rejected
	testutil.ExpectedTrue(t, filteredResults[1])  // epson accepted
}
