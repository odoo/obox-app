package server

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"
)

func TestPrintData_ValidXML_Success(t *testing.T) {
	_, _, err := testutil.StartMockTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
	})
	testutil.ExpectedNoError(t, err)

	s, _ := newTestServer(t, nil)
	printerID := printer.EncodeLANPrinterID("127.0.0.1")
	xmlPayload := `<epos-print><text align="center">ORDER #123</text><cut /></epos-print>`

	url := fmt.Sprintf("/p/%s/cgi-bin/epos/service.cgi", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader([]byte(xmlPayload)))
	req.Header.Set("Content-Type", "text/xml")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)

	body, err := io.ReadAll(resp.Body)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedContains(t, string(body), `success="true"`)
}

func TestPrintData_SchemaError(t *testing.T) {
	s, _ := newTestServer(t, nil)

	invalidPayload := `<invalid>not-an-epos-print</invalid>`
	req := httptest.NewRequest("POST", "/p/any-printer/cgi-bin/epos/service.cgi", bytes.NewReader([]byte(invalidPayload)))
	req.Header.Set("Content-Type", "text/xml")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	testutil.ExpectedContains(t, string(body), `success="false"`)
	testutil.ExpectedContains(t, string(body), `code="SchemaError"`)
}

func TestPrintData_UnreachablePrinter_EX_BADPORT(t *testing.T) {
	s, _ := newTestServer(t, nil)

	printerID := "czpOT05fRVhJU1RFTlRfU0VSSUFMCg"
	xmlPayload := `<epos-print><text>Hello</text></epos-print>`

	url := fmt.Sprintf("/p/%s/cgi-bin/epos/service.cgi", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader([]byte(xmlPayload)))
	req.Header.Set("Content-Type", "text/xml")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	testutil.ExpectedContains(t, string(body), `success="false"`)
	testutil.ExpectedContains(t, string(body), `code="EX_BADPORT"`)
}

func TestPrintLabel_Success(t *testing.T) {
	_, _, err := testutil.StartMockTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
	})
	testutil.ExpectedNoError(t, err)

	s, _ := newTestServer(t, nil)
	printerID := printer.EncodeLANPrinterID("127.0.0.1")
	labelData := []byte("^XA^FDBarcode123^FS^XZ")

	url := fmt.Sprintf("/p/%s/pstprnt", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader(labelData))

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)
}

func TestPrintLabel_EmptyBody_BadRequest(t *testing.T) {
	s, _ := newTestServer(t, nil)

	req := httptest.NewRequest("POST", "/p/any-printer/pstprnt", bytes.NewReader([]byte{}))
	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusBadRequest)
}

func TestPrintLabel_UnreachablePrinter_ServerError(t *testing.T) {
	s, _ := newTestServer(t, nil)

	printerID := "czpOT05fRVhJU1RFTlRfU0VSSUFMCg"
	labelData := []byte("^XA^FDTest^FS^XZ")

	url := fmt.Sprintf("/p/%s/pstprnt", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader(labelData))

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusInternalServerError)
}

func TestCORSHeaders(t *testing.T) {
	s, _ := newTestServer(t, nil)

	req := httptest.NewRequest("OPTIONS", "/cgi-bin/epos/service.cgi", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
}

func TestPrintData_AutoSelectRoute(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected []string
	}{
		{
			name:     "schema error",
			payload:  `<bad></bad>`,
			expected: []string{`code="SchemaError"`},
		},
		{
			name:     "no USB printer",
			payload:  `<epos-print><text align="center">RECEIPT</text><cut /></epos-print>`,
			expected: []string{`success="false"`, `code="EX_BADPORT"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newTestServer(t, nil)

			req := httptest.NewRequest("POST", "/cgi-bin/epos/service.cgi", bytes.NewReader([]byte(tc.payload)))
			req.Header.Set("Content-Type", "text/xml")

			resp, err := s.app.Test(req)
			testutil.ExpectedNoError(t, err)

			body, err := io.ReadAll(resp.Body)
			testutil.ExpectedNoError(t, err)

			for _, expected := range tc.expected {
				testutil.ExpectedContains(t, string(body), expected)
			}
		})
	}
}
