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

// Some thermal printers do not expose the standard USB printer class (0x07)
// and instead use vendor-specific interfaces. These known VID:PID pairs are
// treated as printers even when printer-class detection fails.
var printerRegistry = map[string]Type{
	// Receipt printers
	"2aaf:6015": TypeReceipt, // Essae thermal
	"04b8:0e32": TypeReceipt, // Epson thermal
	"04b8:0202": TypeReceipt, // Epson thermal
	"04b8:0203": TypeReceipt, // Epson thermal
	"04b8:0e27": TypeReceipt, // Epson TM-T83III
	"0d3a:0379": TypeReceipt, // Posiflex
	"2d84:c7c8": TypeReceipt, // Zhuhai Poskey
	"4b43:3830": TypeReceipt, // Caysn
	"0483:5720": TypeReceipt, // STMicroelectronics

	// Label printers
	"0a5f:0187": TypeLabel, // Zebra ZD421
	"195f:0001": TypeLabel, // Godex G500
}

func isKnownPrinter(desc *gousb.DeviceDesc) bool {
	vidPid := strings.ToLower(
		fmt.Sprintf("%04x:%04x", uint16(desc.Vendor), uint16(desc.Product)),
	)
	_, ok := printerRegistry[vidPid]
	return ok
}

func getPrinterType(vidPid string) Type {
	if printerType, ok := printerRegistry[strings.ToLower(vidPid)]; ok {
		return printerType
	}
	return TypeReceipt
}
