package app_actions

import (
	"context"
	"encoding/json"
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/obox"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"
)

func TestExtractPrinterID(t *testing.T) {
	id, ok := extractPrinterID("/usb/v1/printer/ipp_bDoxMjcuMC4wLjE/cgi-bin/epos/service.cgi")
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, id, "ipp_bDoxMjcuMC4wLjE")

	id, ok = extractPrinterID("/usb/v1/printer/usb_czpOT05fRVhJU1RFTlRfU0VSSUFMCg/cgi-bin/epos/service.cgi")
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, id, "usb_czpOT05fRVhJU1RFTlRfU0VSSUFMCg")

	_, ok = extractPrinterID("/usb/v1/printer/")
	testutil.ExpectedFalse(t, ok)

	_, ok = extractPrinterID("/cgi-bin/epos/service.cgi")
	testutil.ExpectedFalse(t, ok)

	_, ok = extractPrinterID("/usb/v1/printer/ipp_123/cgi-bin/epos/service.cgi?debug=1")
	testutil.ExpectedFalse(t, ok)

	_, ok = extractPrinterID("/other/path")
	testutil.ExpectedFalse(t, ok)
}

func TestBuildDeviceList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	err = cfg.AddLanEposPrinter("192.168.1.100")
	testutil.ExpectedNoError(t, err)

	pm := printer.NewManager(cfg)
	list := buildDeviceList(pm)

	found := false
	for _, dev := range list {
		if dev["identifier"] == "ipp_bDoxOTIuMTY4LjEuMTAw" {
			found = true
			testutil.ExpectedEqual(t, dev["type"], "printer")
			testutil.ExpectedEqual(t, dev["name"], "Network - 192.168.1.100")
		}
	}
	testutil.ExpectedTrue(t, found, "Expected to find configured LAN printer in device list")
}

func TestPrintReceiptDirect_SchemaError(t *testing.T) {
	pm := printer.NewManager(nil)

	xmlResult := printReceiptDirect(pm, "/usb/v1/printer/some-printer/cgi-bin/epos/service.cgi", []byte("<invalid>xml</invalid>"))
	testutil.ExpectedContains(t, xmlResult, `success="false"`)
	testutil.ExpectedContains(t, xmlResult, `code="SchemaError"`)
}

func TestPrintReceiptDirect_MalformedPath(t *testing.T) {
	pm := printer.NewManager(nil)

	xmlResult := printReceiptDirect(pm, "/invalid/printer/path", []byte("<epos-print></epos-print>"))
	testutil.ExpectedContains(t, xmlResult, `success="false"`)
	testutil.ExpectedContains(t, xmlResult, `code="EX_BADPORT"`)
}

func TestOboxActionHandler_DiscoverDevices(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	pm := printer.NewManager(cfg)

	handler := OboxActionHandler(pm)

	action := obox.QueueAction{
		UUID: "action-discover",
		Payload: obox.ActionPayload{
			URL: "/odoo/discover_devices",
		},
	}
	res, err := handler(context.Background(), action)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, res)
	_, ok := res.([]map[string]string)
	testutil.ExpectedTrue(t, ok)
}

func TestOboxActionHandler_PrintReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	pm := printer.NewManager(cfg)

	handler := OboxActionHandler(pm)

	actionPrint := obox.QueueAction{
		UUID: "action-print",
		Payload: obox.ActionPayload{
			URL:     "/usb/v1/printer/some-id/cgi-bin/epos/service.cgi",
			Payload: json.RawMessage(`"<invalid>"`),
		},
	}
	resPrint, err := handler(context.Background(), actionPrint)
	testutil.ExpectedNoError(t, err)
	str, ok := resPrint.(string)
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedContains(t, str, `success="false"`)
}

func TestOboxActionHandler_UnsupportedAction(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	pm := printer.NewManager(cfg)

	handler := OboxActionHandler(pm)

	actionUnknown := obox.QueueAction{
		UUID: "action-unknown",
		Payload: obox.ActionPayload{
			URL: "/unknown/action",
		},
	}
	resUnknown, err := handler(context.Background(), actionUnknown)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNil(t, resUnknown)
}
