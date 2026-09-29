package app_actions

import (
	"context"
	"regexp"
	"strings"

	"obox-app/internal/escpos"
	"obox-app/internal/logger"
	"obox-app/internal/obox"
	"obox-app/internal/printer"
)

var printerIDRegex = regexp.MustCompile(`^(?:/usb/v1/printer/|/p/)([^/]+)/cgi-bin/epos/service\.cgi$`)

func OboxActionHandler(pm *printer.Manager) obox.ActionHandler {
	return func(_ context.Context, action obox.QueueAction) (any, error) {
		actionPath := action.Payload.URL

		switch {
		case actionPath == "/odoo/discover_devices":
			return buildDeviceList(pm), nil

		case (strings.HasPrefix(actionPath, "/usb/v1/printer/") || strings.HasPrefix(actionPath, "/p/")) && strings.HasSuffix(actionPath, "/cgi-bin/epos/service.cgi"):
			return printReceiptDirect(pm, actionPath, action.Payload.PayloadBytes()), nil

		default:
			return nil, nil
		}
	}
}

func buildDeviceList(pm *printer.Manager) []map[string]string {
	discovered := pm.DiscoverAllPrinters()
	list := make([]map[string]string, 0, len(discovered.Printers))
	for _, p := range discovered.Printers {
		list = append(list, map[string]string{
			"name":       p.Name,
			"identifier": p.Id,
			"type":       "printer",
		})
	}
	return list
}

func extractPrinterID(actionPath string) (string, bool) {
	matches := printerIDRegex.FindStringSubmatch(actionPath)
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], true
	}
	return "", false
}

func printReceiptDirect(pm *printer.Manager, actionPath string, payload []byte) string {
	printerID, ok := extractPrinterID(actionPath)
	if !ok {
		logger.Errorf("Cannot extract printer ID from action path: %s", actionPath)
		return escpos.NewErrorResponse("EX_BADPORT").String()
	}
	return pm.PrintReceipt(printerID, payload).String()
}
