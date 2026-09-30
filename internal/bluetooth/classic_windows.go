//go:build windows

package bluetooth

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"epos-proxy/internal/logger"

	"golang.org/x/sys/windows"
)

const (
	afBTH          = 32 // AF_BTH
	bthProtoRFCOMM = 3  // BTHPROTO_RFCOMM

	soSndtimeo = 0x1005 // SO_SNDTIMEO
	soRcvtimeo = 0x1006 // SO_RCVTIMEO

	sockaddrBTHSize = 30
)

var (
	ws2_32          = windows.NewLazySystemDLL("ws2_32.dll")
	procSocket      = ws2_32.NewProc("socket")
	procConnect     = ws2_32.NewProc("connect")
	procSend        = ws2_32.NewProc("send")
	procRecv        = ws2_32.NewProc("recv")
	procCloseSocket = ws2_32.NewProc("closesocket")
	procSetsockopt  = ws2_32.NewProc("setsockopt")

	bluetoothAPIs        = windows.NewLazySystemDLL("BluetoothAPIs.dll")
	procBTFindFirst      = bluetoothAPIs.NewProc("BluetoothFindFirstDevice")
	procBTFindNext       = bluetoothAPIs.NewProc("BluetoothFindNextDevice")
	procBTFindClose      = bluetoothAPIs.NewProc("BluetoothFindDeviceClose")
	procBTFindFirstRadio = bluetoothAPIs.NewProc("BluetoothFindFirstRadio")
	procBTFindRadioClose = bluetoothAPIs.NewProc("BluetoothFindRadioClose")
)

var nonPrinterDeviceCoDClasses = []uint32{
	0x01, // Computer (desktop, laptop, server)
	0x02, // Phone (cellular, smartphone)
	0x04, // Audio/Video (headsets, speakers, headphones)
	0x05, // Peripheral (mouse, keyboard, gamepad)
	0x07, // Wearable (watch, etc.)
	0x08, // Toy
	0x09, // Health
}

type btDeviceSearchParams struct {
	dwSize               uint32
	fReturnAuthenticated uint32
	fReturnRemembered    uint32
	fReturnUnknown       uint32
	fReturnConnected     uint32
	fIssueInquiry        uint32
	cTimeoutMultiplier   uint8
	_                    [3]byte
	hRadio               uintptr
}

type winSYSTEMTIME struct {
	Year, Month, DayOfWeek, Day uint16
	Hour, Minute, Second, Ms    uint16
}

type btDeviceInfo struct {
	dwSize          uint32
	_               [4]byte
	Address         uint64
	ulClassOfDevice uint32
	fConnected      uint32
	fRemembered     uint32
	fAuthenticated  uint32
	stLastSeen      winSYSTEMTIME
	stLastUsed      winSYSTEMTIME
	szName          [248]uint16
}

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
	logger.Debug("BT: scanning for paired Bluetooth devices on Windows")
	if err := bluetoothAPIs.Load(); err != nil {
		return nil, fmt.Errorf("BluetoothAPIs.dll unavailable: %w", err)
	}

	params := btDeviceSearchParams{}
	params.dwSize = uint32(unsafe.Sizeof(params))
	params.fReturnAuthenticated = 1
	params.fReturnRemembered = 1
	params.fReturnConnected = 1

	var info btDeviceInfo
	info.dwSize = uint32(unsafe.Sizeof(info))

	handle, _, e := procBTFindFirst.Call(
		uintptr(unsafe.Pointer(&params)),
		uintptr(unsafe.Pointer(&info)),
	)
	const invalidHandle = ^uintptr(0)
	if handle == invalidHandle || handle == 0 {
		if e == windows.ERROR_NO_MORE_ITEMS {
			return nil, nil
		}
		return nil, fmt.Errorf("BluetoothFindFirstDevice failed: %w", e)
	}
	defer procBTFindClose.Call(handle)

	var devices []BluetoothPrinterInfo
	seen := make(map[string]bool)

	for {
		if ctx.Err() != nil {
			return devices, ctx.Err()
		}

		mac, err := ValidateAddress(btAddrToMAC(info.Address))
		if err != nil {
			logger.Errorf("BT: failed to validate address %s: %v", btAddrToMAC(info.Address), err)
		} else {
			name := utf16ToString(info.szName[:])
			if name == "" {
				name = mac
			}

			if !seen[mac] && isPotentialPrinter(name, info.ulClassOfDevice) {
				deviceType := "bluetooth"
				if isPrinterDevice(name) {
					deviceType = "printer"
				}

				seen[mac] = true
				devices = append(devices, BluetoothPrinterInfo{Address: mac, Name: name, Device: deviceType})
			}
		}

		info = btDeviceInfo{}
		info.dwSize = uint32(unsafe.Sizeof(info))
		r, _, _ := procBTFindNext.Call(handle, uintptr(unsafe.Pointer(&info)))
		if r == 0 {
			break
		}
	}

	logger.Debugf("BT: found %d relevant Bluetooth device(s)", len(devices))
	return devices, nil
}

// isPrinterDevice returns true only when we have high-confidence evidence
// this is a printer. We deliberately do NOT use Class-of-Device here:
// CoD's Imaging major class (0x06) covers printers, scanners, cameras and
// fax machines alike, so it can't positively confirm a printer — only a
// name match or (elsewhere) an explicit printer icon can.
func isPrinterDevice(name string) bool {
	return looksLikePrinter(name)
}

// isPotentialPrinter filters out Bluetooth devices that are clearly unrelated
// to printing. CoD is used only to exclude device categories we're certain
// aren't printers (audio, phone, computer, peripheral, wearable, toy, health).
// Everything else — including the ambiguous Imaging class — is kept so the
// user can select it manually.
func isPotentialPrinter(name string, cod uint32) bool {
	// Bluetooth Class of Device (CoD) breakdown:
	// Bits 12-8: Major Device Class
	majorClass := (cod >> 8) & 0x1F
	for _, mc := range nonPrinterDeviceCoDClasses {
		if majorClass == mc {
			return false
		}
	}

	return true
}

func isBluetoothAdapterActive() bool {
	if err := bluetoothAPIs.Load(); err != nil {
		return false
	}

	var params struct {
		dwSize uint32
	}
	params.dwSize = 4
	var hRadio syscall.Handle
	hFind, _, _ := procBTFindFirstRadio.Call(
		uintptr(unsafe.Pointer(&params)),
		uintptr(unsafe.Pointer(&hRadio)),
	)

	const invalidHandle = ^uintptr(0)
	if hFind == 0 || hFind == invalidHandle {
		return false
	}

	_ = syscall.CloseHandle(hRadio)
	_, _, _ = procBTFindRadioClose.Call(hFind)
	return true
}

func checkDependencies() []DependencyStatus {
	return []DependencyStatus{}
}

func macToWindowsBTHAddr(mac string) (uint64, error) {
	parts := strings.Split(strings.ToUpper(mac), ":")
	if len(parts) != 6 {
		return 0, fmt.Errorf("invalid MAC: %s", mac)
	}
	var addr uint64
	for _, p := range parts {
		v, err := parseHexByte(p)
		if err != nil {
			return 0, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		addr = (addr << 8) | uint64(v)
	}
	return addr, nil
}

func parseHexByte(s string) (byte, error) {
	var v uint64
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= uint64(c - '0')
		case c >= 'A' && c <= 'F':
			v |= uint64(c-'A') + 10
		case c >= 'a' && c <= 'f':
			v |= uint64(c-'a') + 10
		default:
			return 0, fmt.Errorf("invalid hex char %q", c)
		}
	}
	return byte(v), nil
}

func makeSockaddrBTH(btAddr uint64, channel uint32) [sockaddrBTHSize]byte {
	var sa [sockaddrBTHSize]byte
	binary.LittleEndian.PutUint16(sa[0:2], afBTH)
	binary.LittleEndian.PutUint64(sa[2:10], btAddr)
	binary.LittleEndian.PutUint32(sa[26:30], channel)
	return sa
}

func btAddrToMAC(addr uint64) string {
	b := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		b[i] = byte(addr & 0xFF)
		addr >>= 8
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[0], b[1], b[2], b[3], b[4], b[5])
}

func utf16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			return windows.UTF16ToString(s[:i])
		}
	}
	return windows.UTF16ToString(s)
}

type windowsBTConn struct {
	sock syscall.Handle
	mac  string
	ch   int
}

func (c *windowsBTConn) LocalAddr() net.Addr {
	return netAddrPlaceholder{net: "rfcomm", addr: fmt.Sprintf("%s/%d", c.mac, c.ch)}
}
func (c *windowsBTConn) RemoteAddr() net.Addr {
	return netAddrPlaceholder{net: "rfcomm", addr: fmt.Sprintf("%s/%d", c.mac, c.ch)}
}

func (c *windowsBTConn) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	r, _, err := procRecv.Call(
		uintptr(c.sock),
		uintptr(unsafe.Pointer(&b[0])),
		uintptr(len(b)),
		0,
	)
	if int32(r) < 0 {
		return 0, fmt.Errorf("recv failed: %w", err)
	}
	return int(r), nil
}

func (c *windowsBTConn) Write(b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		r, _, err := procSend.Call(
			uintptr(c.sock),
			uintptr(unsafe.Pointer(&b[0])),
			uintptr(len(b)),
			0,
		)
		if int32(r) < 0 {
			return total, fmt.Errorf("send failed: %w", err)
		}
		n := int(r)
		if n <= 0 {
			return total, fmt.Errorf("send returned 0 bytes with no error (possible stall)")
		}
		total += n
		b = b[n:]
	}
	return total, nil
}

func (c *windowsBTConn) Close() error {
	r, _, err := procCloseSocket.Call(uintptr(c.sock))
	if r != 0 {
		return fmt.Errorf("closesocket failed: %w", err)
	}
	return nil
}

func (c *windowsBTConn) setTimeoutMS(optname int32, ms int32) error {
	r, _, err := procSetsockopt.Call(
		uintptr(c.sock),
		uintptr(windows.SOL_SOCKET),
		uintptr(optname),
		uintptr(unsafe.Pointer(&ms)),
		uintptr(4),
	)
	if r != 0 {
		return fmt.Errorf("setsockopt failed: %w", err)
	}
	return nil
}

func (c *windowsBTConn) SetDeadline(t time.Time) error {
	_ = c.SetReadDeadline(t)
	return c.SetWriteDeadline(t)
}

func deadlineToMillis(t time.Time) int32 {
	if t.IsZero() {
		return 0 // 0 disables the timeout
	}
	ms := time.Until(t).Milliseconds()
	if ms <= 0 {
		return 1 // already expired: use minimum non-zero timeout
	}
	const maxInt32 = 1<<31 - 1
	if ms > maxInt32 {
		return maxInt32
	}
	return int32(ms)
}

func (c *windowsBTConn) SetReadDeadline(t time.Time) error {
	return c.setTimeoutMS(soRcvtimeo, deadlineToMillis(t))
}

func (c *windowsBTConn) SetWriteDeadline(t time.Time) error {
	return c.setTimeoutMS(soSndtimeo, deadlineToMillis(t))
}

func dialRFCOMM(ctx context.Context, mac string, channel int) (net.Conn, error) {
	btAddr, err := macToWindowsBTHAddr(mac)
	if err != nil {
		return nil, fmt.Errorf("invalid bluetooth MAC %q: %w", mac, err)
	}

	r, _, e := procSocket.Call(afBTH, windows.SOCK_STREAM, bthProtoRFCOMM)
	const invalidSocket = ^uintptr(0)
	if r == invalidSocket {
		return nil, fmt.Errorf("BT socket() failed: %w", e)
	}
	sock := syscall.Handle(r)

	sa := makeSockaddrBTH(btAddr, uint32(channel))

	timeoutMS := int32(btConnectTimeout.Milliseconds())
	procSetsockopt.Call(
		uintptr(sock),
		uintptr(windows.SOL_SOCKET),
		uintptr(soSndtimeo),
		uintptr(unsafe.Pointer(&timeoutMS)),
		4,
	)

	connectCtx, cancel := context.WithTimeout(ctx, btConnectTimeout)
	defer cancel()

	type connectResult struct {
		rc  uintptr
		err error
	}
	// connect() runs in a goroutine so an unresponsive Bluetooth device cannot
	// indefinitely block the caller or printer goroutine.
	ch := make(chan connectResult, 1)
	go func() {
		rc, _, e := procConnect.Call(
			uintptr(sock),
			uintptr(unsafe.Pointer(&sa[0])),
			sockaddrBTHSize,
		)
		ch <- connectResult{rc: rc, err: e}
	}()

	select {
	case <-connectCtx.Done():
		// Best-effort mitigation: closing the socket signals Winsock to abort the connection
		// attempt. Note that on Windows RFCOMM (AF_BTH) sockets, driver behavior when closesocket()
		// races an in-flight blocking connect() varies across Bluetooth stacks — some drivers
		// may hold the OS-level connection attempt open until their internal timeout. The background
		// goroutine will finish on its own and drop its result into the buffered channel ch without
		// leaking memory or blocking the caller.
		procCloseSocket.Call(uintptr(sock))
		return nil, fmt.Errorf("RFCOMM connect to %s channel %d timed out: %w", mac, channel, connectCtx.Err())
	case res := <-ch:
		if res.rc != 0 {
			procCloseSocket.Call(uintptr(sock))
			return nil, fmt.Errorf("RFCOMM connect to %s channel %d failed: %w", mac, channel, res.err)
		}
	}

	return &windowsBTConn{sock: sock, mac: mac, ch: channel}, nil
}

func dialRFCOMMPlatform(ctx context.Context, mac string, cachedChannel int) (net.Conn, error) {
	logger.Debugf("BT/Windows: dialling %s (cached channel %d)", mac, cachedChannel)

	if cachedChannel > 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if conn, err := dialRFCOMM(ctx, mac, cachedChannel); err == nil {
			return conn, nil
		}
	}

	for _, ch := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		if ch == cachedChannel {
			continue
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		logger.Debugf("BT/Windows: probing channel %d for %s", ch, mac)
		if conn, err := dialRFCOMM(ctx, mac, ch); err == nil {
			logger.Debugf("BT/Windows: channel %d succeeded for %s", ch, mac)
			defaultRFCOMMCache.setCachedRFCOMMChannel(mac, ch)
			return conn, nil
		}
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("BT/Windows: no working RFCOMM channel found for %s", mac)
}
