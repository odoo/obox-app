package bluetooth

// netAddrPlaceholder implements net.Addr for Bluetooth network connections across platforms (RFCOMM, BLE).
type netAddrPlaceholder struct {
	net  string
	addr string
}

func (a netAddrPlaceholder) Network() string { return a.net }
func (a netAddrPlaceholder) String() string  { return a.addr }
