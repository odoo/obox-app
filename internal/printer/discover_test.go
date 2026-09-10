package printer

import (
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"

	"github.com/google/gousb"
)

func TestDiscoverAllPrinters_WithUSBAndLANPrinters(t *testing.T) {
	mockPrinters := []*gousb.DeviceDesc{testutil.MockEpsonPrinterDesc(), testutil.MockZebraPrinterDesc()}

	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		var matched []*gousb.Device
		for _, d := range mockPrinters {
			if fn(d) {
				matched = append(matched, &gousb.Device{Desc: d})
			}
		}
		return matched, nil
	})

	cfg := &config.Manager{
		Data: config.AppConfig{
			LANPrinters: []string{"192.168.1.50"},
		},
	}

	mgr := NewManager(0, cfg)
	result := mgr.DiscoverAllPrinters()
	testutil.ExpectedNotNil(t, result.Printers)
	testutil.ExpectedLen(t, result.Printers, 3)

	var usbCount, lanCount int
	for _, p := range result.Printers {
		if p.IsLAN {
			lanCount++
			testutil.ExpectedEqual(t, p.LANIp, "192.168.1.50")
			testutil.ExpectedEqual(t, p.Name, "Network - 192.168.1.50")
		} else {
			usbCount++
			testutil.ExpectedFalse(t, p.IsLAN)
			testutil.ExpectedTrue(t, p.Identifier != "")
		}
	}
	testutil.ExpectedEqual(t, usbCount, 2)
	testutil.ExpectedEqual(t, lanCount, 1)
}
