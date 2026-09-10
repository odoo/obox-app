package obox

import (
	"strings"
)

func (m *Manager) buildDeviceList() []map[string]string {
	discovered := m.printer.DiscoverAllPrinters()
	list := make([]map[string]string, 0, len(discovered.Printers))
	for _, p := range discovered.Printers {
		list = append(list, map[string]string{
			"name":       p.Name,
			"identifier": p.Identifier,
			"type":       "printer",
		})
	}
	return list
}

func extractPrinterID(actionPath string) string {
	path := strings.TrimPrefix(actionPath, "/usb/v1/printer/")
	path = strings.TrimSuffix(path, "/cgi-bin/epos/service.cgi")
	return strings.Trim(path, "/")
}

func (m *Manager) printReceiptDirect(actionPath string, payload []byte) string {
	printerID := extractPrinterID(actionPath)
	return m.printer.PrintReceipt(printerID, payload).String()
}
