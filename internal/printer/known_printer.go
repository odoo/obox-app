package printer

import (
	"fmt"
	"strings"

	"github.com/google/gousb"
)

// Type is what a printer produces, which decides how job data is framed.
type Type string

const (
	TypeReceipt Type = "receipt"
	TypeLabel   Type = "label"
)

// Protocol defines the command protocol supported by the printer.
type Protocol string

const (
	ProtocolESCPOS        Protocol = "ESCPOS"
	ProtocolESCPOSPartial Protocol = "ESCPOS_PARTIAL"
)

const (
	DefaultPrinterType          = TypeReceipt
	DefaultPrinterProtocol      = ProtocolESCPOS
	DefaultPrinterBottomPadding = 0
)

func ParseProtocol(raw string) Protocol {
	switch {
	case strings.EqualFold(raw, string(ProtocolESCPOSPartial)):
		return ProtocolESCPOSPartial
	default:
		return DefaultPrinterProtocol
	}
}

type KnownPrinter struct {
	Type          Type
	Protocol      Protocol
	BottomPadding int
}

// Some thermal printers do not expose the standard USB printer class (0x07)
// and instead use vendor-specific interfaces. These known VID:PID pairs are
// treated as printers even when printer-class detection fails.
var printerRegistry = map[string]KnownPrinter{
	// Receipt printers
	"2aaf:6015": {},                                                    // Essae thermal
	"04b8:0e32": {},                                                    // Epson thermal
	"04b8:0202": {},                                                    // Epson thermal
	"04b8:0203": {},                                                    // Epson thermal
	"04b8:0e27": {},                                                    // Epson TM-T83III
	"2d84:c7c8": {},                                                    // Zhuhai Poskey
	"4b43:3830": {},                                                    // Caysn
	"0456:0808": {Protocol: ProtocolESCPOSPartial, BottomPadding: 120}, // ATPOS 58mm Thermal Printer (partially support escpos)
	"0483:5720": {Protocol: ProtocolESCPOSPartial},                     // STMicroelectronics (partially support escpos)

	// Label printers
	"0a5f:0187": {Type: TypeLabel}, // Zebra ZD421
	"195f:0001": {Type: TypeLabel}, // Godex G500
}

func isKnownPrinter(desc *gousb.DeviceDesc) bool {
	vidPid := strings.ToLower(fmt.Sprintf("%04x:%04x", uint16(desc.Vendor), uint16(desc.Product)))
	_, ok := printerRegistry[vidPid]
	return ok
}

func getPrinterType(vidPid string) Type {
	return getKnownPrinter(vidPid).Type
}

func getKnownPrinter(vidPid string) KnownPrinter {
	printer := printerRegistry[strings.ToLower(vidPid)]
	if printer.Type == "" {
		printer.Type = DefaultPrinterType
	}

	if printer.Type == TypeReceipt {
		if printer.Protocol == "" {
			printer.Protocol = DefaultPrinterProtocol
		}
		if printer.BottomPadding == 0 {
			printer.BottomPadding = DefaultPrinterBottomPadding
		}
	}

	return printer
}

func getKnownPrinterByID(id string) (Protocol, int) {
	decoded, err := decodePrinterID(id)
	if err != nil || decoded == nil {
		return DefaultPrinterProtocol, 0
	}
	kp := getKnownPrinter(decoded.VidPid)
	return kp.Protocol, kp.BottomPadding
}
