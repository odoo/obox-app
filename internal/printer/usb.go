package printer

import (
	"fmt"

	"obox-app/internal/logger"

	"github.com/google/gousb"
)

// Info describes a USB printer that was successfully probed.
type Info struct {
	Id   string
	Name string
	Type Type
}

// UnavailableInfo describes a device that looks like a printer but could
// not be opened, usually because of permissions or a driver holding it.
type UnavailableInfo struct {
	Name  string
	Error string
}

type Printers struct {
	Available   []Info
	Unavailable []UnavailableInfo
}

// EndpointInfo locates the bulk OUT endpoint used to write to a device.
type EndpointInfo struct {
	config           int
	iFace            int
	alternateSetting int
	outEndpoint      int
}

// LibUsbPrinter is the raw identity read off a device during a scan.
type LibUsbPrinter struct {
	Serial   string
	Path     string
	Name     string
	VidPid   string
	DeviceId DeviceID
}

func ListUSBPrinters() (*Printers, error) {
	logger.Debug("Starting USB printer detection")
	ctx := gousb.NewContext()
	defer func(ctx *gousb.Context) {
		_ = ctx.Close()
	}(ctx)

	var keys []string
	// First list all without opening devices, to avoid permission errors on some platforms
	var descriptors []gousb.DeviceDesc
	_, err := openDevices(ctx, func(desc *gousb.DeviceDesc) bool {
		if _, supported := findPrinterEndpoint(desc); supported {
			descriptors = append(descriptors, *desc)
			keys = append(keys, fingerprintKey(desc))
		}
		return false
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open USB devices for listing: %w", err)
	}

	if !usbCache.HasChanged(keys) && !usbCache.HasUnavailable() {
		logger.Debugf("USB unchanged → using cache")
		return &Printers{Available: usbCache.Get()}, nil
	}

	logger.Debugf("USB changed → rescanning devices")

	result := &Printers{
		Available:   make([]Info, 0),
		Unavailable: make([]UnavailableInfo, 0),
	}
	for _, desc := range descriptors {
		info, err := GetPrinterInfo(ctx, &desc)
		if err != nil {
			// Device is not accessible, likely due to permissions / drivers.
			vid := fmt.Sprintf("%04X", uint16(desc.Vendor))
			pid := fmt.Sprintf("%04X", uint16(desc.Product))
			result.Unavailable = append(result.Unavailable, UnavailableInfo{
				Name:  getPrinterFriendlyName(vid, pid),
				Error: err.Error(),
			})
		} else if info != nil {
			id, err := encodePrinterID(info)
			if err != nil {
				logger.Errorf("failed to encode printer ID: %v", err)
				continue
			}
			result.Available = append(result.Available, Info{
				Id:   id,
				Name: info.Name,
				Type: getPrinterType(info.VidPid),
			})
		}
	}

	usbCache.Update(keys, result.Available, result.Unavailable)
	return result, nil
}

func GetPrinterInfo(ctx *gousb.Context, descToFind *gousb.DeviceDesc) (*LibUsbPrinter, error) {
	logger.Debugf("Attempting to get info for USB device: Bus %d, Address %d, Vendor %04X, Product %04X", descToFind.Bus, descToFind.Address, uint16(descToFind.Vendor), uint16(descToFind.Product))
	var found bool
	devices, err := openDevices(ctx, func(desc *gousb.DeviceDesc) bool {
		if found {
			return false
		}
		if descToFind.Bus != desc.Bus || descToFind.Address != desc.Address ||
			descToFind.Vendor != desc.Vendor || descToFind.Product != desc.Product {
			return false
		}
		found = true
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open USB device for info retrieval: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil
	}

	defer func() {
		for _, d := range devices {
			_ = d.Close()
		}
	}()

	device := devices[0]
	deviceID := getPrinterDeviceID(device)
	info := LibUsbPrinter{}
	productName, _ := device.Product()
	vendorName, _ := device.Manufacturer()
	serial, _ := device.SerialNumber()

	if productName == "" {
		if mdl, ok := deviceID["MDL"]; ok {
			productName = mdl
		} else {
			productName = fmt.Sprintf("PID: %04X", uint16(descToFind.Product))
		}
	}

	if vendorName == "" {
		if vendor, ok := deviceID["MFG"]; ok {
			vendorName = vendor
		} else {
			vendorName = fmt.Sprintf("VID: %04X", uint16(descToFind.Vendor))
		}
	}

	info.Name = fmt.Sprintf("%s %s", vendorName, productName)
	info.Serial = serial
	info.Path = pathToString(descToFind)
	info.VidPid = fmt.Sprintf("%04X:%04X", uint16(descToFind.Vendor), uint16(descToFind.Product))
	info.DeviceId = deviceID
	logger.Debugf("USB printer: %s (Serial: %s)", info.Name, info.Serial)
	return &info, nil
}

func fingerprintKey(desc *gousb.DeviceDesc) string {
	return fmt.Sprintf("%d-%d-%04X:%04X-%s",
		desc.Bus,
		desc.Address,
		desc.Vendor,
		desc.Product,
		pathToString(desc),
	)
}

func findPrinterEndpoint(desc *gousb.DeviceDesc) (EndpointInfo, bool) {
	missingBulkOut := false
	logger.Debugf("Finding printer endpoint for device: %v", desc)
	for cfgNum, cfg := range desc.Configs {
		for _, iFace := range cfg.Interfaces {
			for _, alt := range iFace.AltSettings {
				if alt.Class != gousb.ClassPrinter && !isKnownPrinter(desc) {
					continue
				}
				if epNum, ok := matchBulkOutEndpoint(alt); ok {
					return EndpointInfo{
						config:           cfgNum,
						iFace:            iFace.Number,
						alternateSetting: alt.Alternate,
						outEndpoint:      epNum,
					}, true
				} else {
					missingBulkOut = true
				}
			}
		}
	}
	if missingBulkOut {
		logger.Warnf("Printer device rejected during endpoint matching: VID=%s, PID=%s, reason=no bulk OUT endpoint found", desc.Vendor, desc.Product)
	}
	return EndpointInfo{}, false
}

func matchBulkOutEndpoint(alt gousb.InterfaceSetting) (int, bool) {
	for _, ep := range alt.Endpoints {
		if ep.Direction == gousb.EndpointDirectionOut &&
			ep.TransferType == gousb.TransferTypeBulk {
			return ep.Number, true
		}
	}
	return 0, false
}
