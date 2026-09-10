package obox

import (
	"encoding/json"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestObox_BuildDeviceList(t *testing.T) {
	m, _ := createTestModule(t)
	err := m.cfg.AddLanEposPrinter("192.168.1.100")
	testutil.ExpectedNoError(t, err)

	for _, dev := range m.buildDeviceList() {
		testutil.ExpectedEqual(t, dev["type"], "printer")
		testutil.ExpectedEqual(t, dev["name"], "Network - 192.168.1.100")
		testutil.ExpectedEqual(t, dev["identifier"], "ipp_bDoxOTIuMTY4LjEuMTAw")
	}
}

func TestObox_ExtractPrinterID(t *testing.T) {
	testutil.ExpectedEqual(t, extractPrinterID("/usb/v1/printer/ipp_bDoxMjcuMC4wLjE/cgi-bin/epos/service.cgi"), "ipp_bDoxMjcuMC4wLjE")
	testutil.ExpectedEqual(t, extractPrinterID("/cgi-bin/epos/service.cgi"), "")
}

func TestObox_ActionPayload_PayloadBytes(t *testing.T) {
	// JSON string payload
	p1 := ActionPayload{Payload: json.RawMessage(`"<epos-print></epos-print>"`)}
	testutil.ExpectedEqual(t, string(p1.PayloadBytes()), "<epos-print></epos-print>")
	// Raw bytes payload
	p2 := ActionPayload{Payload: json.RawMessage(`<epos-print></epos-print>`)}
	testutil.ExpectedEqual(t, string(p2.PayloadBytes()), "<epos-print></epos-print>")
	// Null or empty
	p3 := ActionPayload{Payload: json.RawMessage(`null`)}
	testutil.ExpectedNil(t, p3.PayloadBytes())
	// Nil payload
	p4 := ActionPayload{Payload: nil}
	testutil.ExpectedNil(t, p4.PayloadBytes())
}

func TestObox_DirectPrintReceipt_SchemaError(t *testing.T) {
	m, _ := createTestModule(t)
	xmlResult := m.printReceiptDirect("/usb/v1/printer/some-printer/cgi-bin/epos/service.cgi", json.RawMessage(`"<invalid>xml</invalid>"`))
	testutil.ExpectedContains(t, xmlResult, `success="false"`)
	testutil.ExpectedContains(t, xmlResult, `code="SchemaError"`)
}

func TestObox_DirectPrintReceipt_BadPort(t *testing.T) {
	m, _ := createTestModule(t)
	xmlResult := m.printReceiptDirect("/usb/v1/printer/czpOT05fRVhJU1RFTlRfU0VSSUFMCg/cgi-bin/epos/service.cgi", json.RawMessage(`"<epos-print><text>hello</text></epos-print>"`))
	testutil.ExpectedContains(t, xmlResult, `success="false"`)
	testutil.ExpectedContains(t, xmlResult, `code="EX_BADPORT"`)
}
