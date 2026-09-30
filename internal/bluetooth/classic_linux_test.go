//go:build linux

package bluetooth

import (
	"context"
	"os"
	"testing"
	"time"

	"epos-proxy/internal/testutil"
)

func TestLinux_CheckDependencies(t *testing.T) {
	deps := checkDependencies()
	for _, d := range deps {
		testutil.ExpectedTrue(t, d.Name != "", "dependency Name should not be empty")
		testutil.ExpectedTrue(t, d.InstallCmd != "", "dependency InstallCmd should not be empty")
	}
}

func TestLinux_ClassicTransport(t *testing.T) {
	transport := &classicTransport{}
	testutil.ExpectedEqual(t, transport.name(), "Classic")

	// Verify scan with cancelled context behaves safely
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = transport.scan(ctx)
}

func TestLinux_SerialConn(t *testing.T) {
	r, w, err := os.Pipe()
	testutil.ExpectedNoError(t, err)
	defer r.Close()
	defer w.Close()

	conn := &serialConn{
		f:    w,
		path: "00:11:22:33:44:55/1",
	}

	testutil.ExpectedEqual(t, conn.LocalAddr().Network(), "rfcomm")
	testutil.ExpectedEqual(t, conn.LocalAddr().String(), "00:11:22:33:44:55/1")
	testutil.ExpectedEqual(t, conn.RemoteAddr().Network(), "rfcomm")
	testutil.ExpectedEqual(t, conn.RemoteAddr().String(), "00:11:22:33:44:55/1")

	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))

	n, err := conn.Write([]byte("ping"))
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, n, 4)
}

func TestLinux_IsPrinterDevice(t *testing.T) {
	// Case 1: 80Printer-1C31 (has SPP and Class 0x00800610 and Name 80Printer-1C31, but no Icon)
	printer80Info := `device 28:d4:1e:2d:1c:31 (public)
	name: 80printer-1c31
	alias: 80printer-1c31
	class: 0x00800610 (8390160)
	paired: yes
	uuid: serial port               (00001101-0000-1000-8000-00805f9b34fb)`
	testutil.ExpectedTrue(t, isPrinterDevice("80Printer-1C31", printer80Info), "80Printer-1C31 should be identified as a printer")

	// Case 2: SR588 (has SPP, Icon: printer, and Class 0x00040680)
	sr588Info := `device c8:47:8c:38:31:dc (public)
	name: sr588
	class: 0x00040680 (263808)
	icon: printer
	paired: yes
	uuid: serial port               (00001101-0000-1000-8000-00805f9b34fb)`
	testutil.ExpectedTrue(t, isPrinterDevice("SR588", sr588Info), "SR588 should be identified as a printer")

	// Case 3: DuoPods headset (has SPP, but Audio class and no printer keywords or icon)
	headsetInfo := `device 41:42:ff:15:18:8a (public)
	name: mivi duopods storm
	class: 0x00240404 (2360324)
	icon: audio-headset
	paired: yes
	uuid: serial port               (00001101-0000-1000-8000-00805f9b34fb)
	uuid: audio sink                (0000110b-0000-1000-8000-00805f9b34fb)`
	testutil.ExpectedFalse(t, isPrinterDevice("Mivi DuoPods Storm", headsetInfo), "Headset should not be identified as a printer")

	// Case 4: Headset with SPP but no printer keywords or icon
	testutil.ExpectedFalse(t, isPrinterDevice("Generic Audio", headsetInfo), "Generic audio device should not be identified as a printer")
}

func TestLinux_IsPotentialPrinter(t *testing.T) {
	// Case 1: Explicit printer icon is always potential printer
	printerInfo := `icon: printer`
	testutil.ExpectedTrue(t, isPotentialPrinter(printerInfo), "Explicit printer should be potential printer")

	// Case 2: No SPP support is excluded
	noSPPInfo := `device 11:22:33:44:55:66 (public)
	name: thermal printer
	class: 0x00040680
	paired: yes`
	testutil.ExpectedFalse(t, isPotentialPrinter(noSPPInfo), "Device without SPP should not be potential printer")

	// Case 3: Excluded device categories (headset, scanner, phone, computer)
	headsetInfo := `icon: audio-headset
	uuid: serial port (00001101-0000-1000-8000-00805f9b34fb)`
	testutil.ExpectedFalse(t, isPotentialPrinter(headsetInfo), "Audio headset should not be potential printer")

	scannerInfo := `icon: scanner
	uuid: serial port (00001101-0000-1000-8000-00805f9b34fb)`
	testutil.ExpectedFalse(t, isPotentialPrinter(scannerInfo), "Scanner should not be potential printer")

	// Case 4: Unknown device exposing SPP is kept for manual user selection
	unknownSPP := `uuid: serial port (00001101-0000-1000-8000-00805f9b34fb)`
	testutil.ExpectedTrue(t, isPotentialPrinter(unknownSPP), "Unknown device with SPP should be kept for user selection")
}
