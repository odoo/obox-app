package printer

import (
	"errors"
	"testing"

	"obox-app/internal/testutil"

	"github.com/google/gousb"
)

func TestFindPrinterEndpoint(t *testing.T) {
	// Standard USB printer class device (Class 7) with Bulk OUT endpoint
	descPrinterClass := testutil.MockEpsonPrinterDesc()
	epInfo, ok := findPrinterEndpoint(descPrinterClass)
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, epInfo.outEndpoint, 1)
	testutil.ExpectedEqual(t, epInfo.iFace, 0)
	testutil.ExpectedEqual(t, epInfo.config, 1)

	// Known printer (e.g. 0a5f:0187 Zebra) with vendor-specific class (Class 0xFF)
	descKnownVendorClass := testutil.MockZebraPrinterDesc()
	epInfoKnown, okKnown := findPrinterEndpoint(descKnownVendorClass)
	testutil.ExpectedTrue(t, okKnown)
	testutil.ExpectedEqual(t, epInfoKnown.outEndpoint, 2)

	// Non-printer device (e.g. Mass Storage Class 8, unknown VID/PID)
	descNonPrinter := testutil.MockMassStorageDesc()
	_, okNonPrinter := findPrinterEndpoint(descNonPrinter)
	testutil.ExpectedFalse(t, okNonPrinter)

	// Printer class device missing Bulk OUT endpoint
	descMissingBulk := testutil.MockMissingBulkPrinterDesc()
	_, okMissing := findPrinterEndpoint(descMissingBulk)
	testutil.ExpectedFalse(t, okMissing)

	// Multi-interface device where 2nd interface is a Printer with Bulk OUT
	descMultiInterface := testutil.MockMultiInterfacePrinterDesc()
	epInfoMulti, okMulti := findPrinterEndpoint(descMultiInterface)
	testutil.ExpectedTrue(t, okMulti)
	testutil.ExpectedEqual(t, epInfoMulti.iFace, 1)
	testutil.ExpectedEqual(t, epInfoMulti.outEndpoint, 2)
}

func TestFingerprintKey(t *testing.T) {
	desc1 := &gousb.DeviceDesc{
		Bus:     1,
		Address: 4,
		Vendor:  0x04B8,
		Product: 0x0202,
		Path:    []int{1, 2},
	}

	desc2 := &gousb.DeviceDesc{
		Bus:     1,
		Address: 5,
		Vendor:  0x04B8,
		Product: 0x0202,
		Path:    []int{1, 2},
	}

	key1 := fingerprintKey(desc1)
	key2 := fingerprintKey(desc2)

	testutil.ExpectedTrue(t, key1 != "")
	testutil.ExpectedTrue(t, key2 != "")
	testutil.ExpectedEqual(t, key1, fingerprintKey(desc1))
	testutil.ExpectedNotEqual(t, key1, key2)
}

func mockOpenDevices(t testing.TB, mockFn func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error)) {
	t.Helper()
	oldOpenDevices := openDevices
	openDevices = mockFn
	t.Cleanup(func() {
		openDevices = oldOpenDevices
		usbCache.Update(nil, nil, nil)
	})
}

func TestListUSBPrinters_WithMockOpenDevices(t *testing.T) {
	mockPrinters := []*gousb.DeviceDesc{
		testutil.MockEpsonPrinterDesc(),
		testutil.MockZebraPrinterDesc(),
		testutil.MockMassStorageDesc(),
	}

	callCount := 0
	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		var matched []*gousb.Device
		for _, d := range mockPrinters {
			if fn(d) {
				matched = append(matched, &gousb.Device{Desc: d})
			}
		}
		callCount++
		return matched, nil
	})

	res, err := ListUSBPrinters()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, res)
	testutil.ExpectedLen(t, res.Available, 2)
	testutil.ExpectedLen(t, res.Unavailable, 0)

	// Verify the mocked printers are returned correctly
	testutil.ExpectedEqual(t, res.Available[0].Name, "VID: 04B8 PID: 0202")
	testutil.ExpectedEqual(t, res.Available[0].Type, TypeReceipt)
	id0, err := decodePrinterID(res.Available[0].Id)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, id0.VidPid, "04B8:0202")

	testutil.ExpectedEqual(t, res.Available[1].Name, "VID: 0A5F PID: 0187")
	testutil.ExpectedEqual(t, res.Available[1].Type, TypeLabel)
	id1, err := decodePrinterID(res.Available[1].Id)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, id1.VidPid, "0A5F:0187")

	// First scan calls openDevices once for scanning descriptors + once per printer in GetPrinterInfo (1 + 2 = 3)
	testutil.ExpectedEqual(t, callCount, 3)

	// Second invocation should hit cache and return identical printers
	cachedRes, err := ListUSBPrinters()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, cachedRes)
	testutil.ExpectedLen(t, cachedRes.Available, 2)
	testutil.ExpectedLen(t, cachedRes.Unavailable, 0)
	testutil.ExpectedEqual(t, cachedRes.Available[0].Id, res.Available[0].Id)
	testutil.ExpectedEqual(t, cachedRes.Available[1].Id, res.Available[1].Id)

	// Second scan only runs the descriptor check (1 call) and skips GetPrinterInfo calls due to cache (3 + 1 = 4)
	testutil.ExpectedEqual(t, callCount, 4)
}

func TestListUSBPrinters_OpenDevicesError(t *testing.T) {
	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		return nil, errors.New("simulated libusb error")
	})

	res, err := ListUSBPrinters()
	testutil.ExpectedError(t, err)
	testutil.ExpectedNil(t, res)
}

func TestListUSBPrinters_UnavailableDevice(t *testing.T) {
	mockPrinters := []*gousb.DeviceDesc{
		testutil.MockEpsonPrinterDesc(),
	}

	callCount := 0
	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		callCount++
		if callCount == 1 {
			// First scan call lists descriptors
			for _, d := range mockPrinters {
				_ = fn(d)
			}
			return nil, nil
		}
		// Second call (in GetPrinterInfo) fails opening device
		return nil, errors.New("device locked by another process")
	})

	res, err := ListUSBPrinters()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, res)
	testutil.ExpectedLen(t, res.Unavailable, 1)
	testutil.ExpectedTrue(t, res.Unavailable[0].Name != "")
	testutil.ExpectedEqual(t, res.Unavailable[0].Error, "failed to open USB device for info retrieval: device locked by another process")
}

func TestGetPrinterInfo_Direct(t *testing.T) {
	epsonDesc := testutil.MockEpsonPrinterDesc()

	// 1. Device found and opened
	mockOpenDevices(t, func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		if fn(epsonDesc) {
			return []*gousb.Device{{Desc: epsonDesc}}, nil
		}
		return nil, nil
	})

	ctx := gousb.NewContext()
	defer ctx.Close()

	info, err := GetPrinterInfo(ctx, epsonDesc)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, info)
	testutil.ExpectedEqual(t, info.VidPid, "04B8:0202")
	testutil.ExpectedEqual(t, info.Path, "1.1.2")

	// 2. Open error
	openDevices = func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		return nil, errors.New("cannot open device")
	}

	infoErr, err := GetPrinterInfo(ctx, epsonDesc)
	testutil.ExpectedError(t, err)
	testutil.ExpectedNil(t, infoErr)

	// 3. Device not found (empty slice)
	openDevices = func(ctx *gousb.Context, fn func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
		return nil, nil
	}

	infoNotFound, err := GetPrinterInfo(ctx, epsonDesc)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNil(t, infoNotFound)
}
