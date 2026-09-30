//go:build linux

package bluetooth

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"epos-proxy/internal/logger"

	"golang.org/x/sys/unix"
)

// BlueZ reports a device category through the Icon field.
// Keep only devices whose type could reasonably be a printer.
var excludedIcons = []string{
	"icon: audio-headset",
	"icon: audio-card",
	"icon: phone",
	"icon: computer",
	"icon: input-keyboard",
	"icon: input-mouse",
	"icon: scanner",
	"icon: camera",
	"icon: modem",
	"icon: gamepad",
	"icon: tv",
	"icon: wearable",
}

// serialConn adapts an *os.File (RFCOMM socket file descriptor)
// to implement Go's standard net.Conn interface on Linux.
type serialConn struct {
	f    *os.File
	path string
}

func (c *serialConn) Read(b []byte) (int, error)  { return c.f.Read(b) }
func (c *serialConn) Write(b []byte) (int, error) { return c.f.Write(b) }
func (c *serialConn) Close() error                { return c.f.Close() }

func (c *serialConn) LocalAddr() net.Addr {
	return netAddrPlaceholder{net: "rfcomm", addr: c.path}
}
func (c *serialConn) RemoteAddr() net.Addr {
	return netAddrPlaceholder{net: "rfcomm", addr: c.path}
}

func (c *serialConn) SetDeadline(t time.Time) error      { return c.f.SetDeadline(t) }
func (c *serialConn) SetReadDeadline(t time.Time) error  { return c.f.SetReadDeadline(t) }
func (c *serialConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

type classicTransport struct{}

func (t *classicTransport) name() string {
	return "Classic"
}

func (t *classicTransport) isAvailable() bool {
	return isBluetoothAdapterActive()
}

func (t *classicTransport) dial(ctx context.Context, address string) (net.Conn, error) {
	channel := defaultRFCOMMCache.getCachedRFCOMMChannel(address)
	return dialRFCOMMPlatform(ctx, address, channel)
}

func (t *classicTransport) scan(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	return scanPairedPrinters(ctx)
}

// scanPairedPrinters lists paired Bluetooth devices that could be used as printers.
// Devices are classified as either "printer" or "bluetooth" (unknown).
func scanPairedPrinters(ctx context.Context) ([]BluetoothPrinterInfo, error) {
	logger.Debug("BT: scanning for paired Bluetooth devices on Linux")

	out, err := exec.CommandContext(ctx, "bluetoothctl", "devices").Output()
	if err != nil {
		return nil, fmt.Errorf("bluetoothctl devices failed: %w — is bluez installed?", err)
	}

	var devices []BluetoothPrinterInfo
	seen := make(map[string]bool)

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Device ") {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}

		mac, err := ValidateAddress(parts[1])
		if err != nil {
			logger.Errorf("BT: failed to validate address %s: %v", parts[1], err)
			continue
		}

		if seen[mac] {
			continue
		}

		name := "Unknown"
		if len(parts) == 3 {
			name = strings.TrimSpace(parts[2])
		}

		infoOut, err := exec.CommandContext(ctx, "bluetoothctl", "info", mac).Output()
		if err != nil {
			logger.Warnf("BT: failed to get info for %s: %v", mac, err)
			continue
		}

		info := strings.ToLower(string(infoOut))

		// Ignore devices that are clearly not relevant to printing.
		if !isPotentialPrinter(info) {
			continue
		}

		deviceType := "bluetooth"
		if isPrinterDevice(name, info) {
			deviceType = "printer"
		}

		seen[mac] = true
		devices = append(devices, BluetoothPrinterInfo{
			Address: mac,
			Name:    name,
			Device:  deviceType,
		})
	}

	logger.Debugf("BT: found %d relevant Bluetooth device(s)", len(devices))
	return devices, nil
}

// isPotentialPrinter filters out Bluetooth devices that are clearly unrelated
// to printing. Unknown devices that expose SPP are kept so the user can select
// them manually.
func isPotentialPrinter(info string) bool {
	// An explicit printer is always relevant.
	if strings.Contains(info, "icon: printer") {
		return true
	}

	// We communicate with Bluetooth printers through SPP.
	if !supportsSPP(info) {
		return false
	}

	for _, icon := range excludedIcons {
		if strings.Contains(info, icon) {
			return false
		}
	}

	return true
}

func isPrinterDevice(name string, info string) bool {
	if strings.Contains(info, "icon: printer") {
		return true
	}

	// Some printers don't report the printer icon but have a recognizable name.
	if looksLikePrinter(name) {
		return true
	}

	return false
}

func supportsSPP(info string) bool {
	return strings.Contains(info, "uuid: serial port") ||
		strings.Contains(info, "00001101-0000-1000-8000-00805f9b34fb")
}

func isBluetoothAdapterActive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "bluetoothctl", "show").Output()
	if err != nil {
		logger.Errorf("BT/classic: failed to check if bluetooth adapter is active: %v", err)
		return false
	}
	return strings.Contains(string(out), "Powered: yes")
}

func checkDependencies() []DependencyStatus {
	deps := []DependencyStatus{}

	// Check bluetoothctl (bluez)
	_, err := exec.LookPath("bluetoothctl")
	if err != nil {
		deps = append(deps, DependencyStatus{
			Name:        "bluez",
			InstallCmd:  "sudo apt-get install bluez",
			Description: "Required to scan for Bluetooth devices and manage connections",
		})
	}

	return deps
}

// dialRFCOMMPlatform attempts to connect to a Bluetooth device via RFCOMM socket,
// trying the cached channel first, then probing channels 1–8.
func dialRFCOMMPlatform(ctx context.Context, mac string, cachedChannel int) (net.Conn, error) {
	logger.Debugf("BT/RFCOMM: dialling %s (cached channel %d)", mac, cachedChannel)

	// Step 0: Try cached channel first if it exists.
	if cachedChannel > 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		logger.Debugf("BT/RFCOMM: trying cached channel %d for %s", cachedChannel, mac)
		if conn, err := dialRFCOMM(mac, cachedChannel); err == nil {
			logger.Debugf("BT/RFCOMM: connected successfully on cached channel %d for %s", cachedChannel, mac)
			return conn, nil
		}
		logger.Warnf("BT/RFCOMM: connection on cached channel %d failed for %s", cachedChannel, mac)
	}

	// Step 1: Channel probing.
	for _, ch := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		if ch == cachedChannel {
			continue
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		logger.Debugf("BT/RFCOMM: probing channel %d for %s", ch, mac)
		if conn, err := dialRFCOMM(mac, ch); err == nil {
			logger.Debugf("BT/RFCOMM: channel probe succeeded on channel %d for %s", ch, mac)
			defaultRFCOMMCache.setCachedRFCOMMChannel(mac, ch)
			return conn, nil
		}
		logger.Debugf("BT/RFCOMM: channel %d probe failed for %s", ch, mac)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("BT/RFCOMM: all connection strategies failed for %s", mac)
}

func dialRFCOMM(mac string, channel int) (net.Conn, error) {
	addr, err := parseMACToBytes(mac)
	if err != nil {
		return nil, fmt.Errorf("invalid bluetooth MAC %q: %w", mac, err)
	}

	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_STREAM, unix.BTPROTO_RFCOMM)
	if err != nil {
		return nil, fmt.Errorf("create RFCOMM socket failed: %w", err)
	}
	cleanup := func() { _ = unix.Close(fd) }

	unix.CloseOnExec(fd)
	if err := unix.SetNonblock(fd, true); err != nil {
		cleanup()
		return nil, fmt.Errorf("set nonblocking mode failed: %w", err)
	}

	_ = unix.SetsockoptLinger(fd, unix.SOL_SOCKET, unix.SO_LINGER, &unix.Linger{Onoff: 1, Linger: 1})

	sa := &unix.SockaddrRFCOMM{Addr: addr, Channel: uint8(channel)}
	err = unix.Connect(fd, sa)
	if err != nil && err != unix.EINPROGRESS && err != unix.EAGAIN {
		cleanup()
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: %w", mac, channel, err)
	}

	pollFds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
	var n int
	for {
		var pollErr error
		n, pollErr = unix.Poll(pollFds, int(btConnectTimeout.Milliseconds()))
		if pollErr == unix.EINTR {
			continue // signal interrupted poll, retry
		}
		if pollErr != nil {
			cleanup()
			return nil, fmt.Errorf("RFCOMM poll failed for %s channel %d: %w", mac, channel, pollErr)
		}
		break
	}
	if n == 0 {
		cleanup()
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d timed out", mac, channel)
	}

	if pollFds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
		soErr, _ := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
		cleanup()
		if soErr != 0 {
			return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: errno=%d (%s)",
				mac, channel, soErr, syscall.Errno(soErr).Error())
		}
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed", mac, channel)
	}

	soErr, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("SO_ERROR check failed: %w", err)
	}
	if soErr != 0 {
		cleanup()
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: errno=%d (%s)", mac, channel, soErr, syscall.Errno(soErr).Error())
	}

	file := os.NewFile(uintptr(fd), fmt.Sprintf("rfcomm-%s-%d", mac, channel))
	if file == nil {
		cleanup()
		return nil, fmt.Errorf("failed to create os.File from RFCOMM socket")
	}

	return &serialConn{f: file, path: fmt.Sprintf("%s/%d", mac, channel)}, nil
}
