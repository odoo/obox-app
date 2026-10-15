package printer

import (
	"net"
	"testing"

	"obox-app/internal/config"
	"obox-app/internal/testutil"
)

func TestValidateIPAddress(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{name: "valid IPv4", input: "192.168.1.50", expected: "192.168.1.50", expectErr: false},
		{name: "valid IPv4 with whitespace", input: "  10.0.0.1  ", expected: "10.0.0.1", expectErr: false},
		{name: "valid IPv6", input: "::1", expected: "::1", expectErr: false},
		{name: "empty string", input: "   ", expected: "", expectErr: true},
		{name: "invalid format", input: "999.999.999.999", expected: "", expectErr: true},
		{name: "alphabetic string", input: "not.an.ip.address", expected: "", expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateIPAddress(tc.input)
			if tc.expectErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
			}
			testutil.ExpectedEqual(t, got, tc.expected)
		})
	}
}

func TestCheckLANPrinter_SuccessAndOffline(t *testing.T) {
	// 1. Offline IP / unreachable port
	err := CheckLANPrinter("127.0.0.254")
	testutil.ExpectedError(t, err)

	// 2. Mock a live TCP server on port 9100
	_, _, err = testutil.StartMockTCPServer(t, func(conn net.Conn) {
		_ = conn.Close()
	})
	testutil.ExpectedNoError(t, err)

	err = CheckLANPrinter("127.0.0.1")
	testutil.ExpectedNoError(t, err)
}

func TestListLANPrinters(t *testing.T) {
	cfg := &config.Manager{
		Data: config.AppConfig{
			LANPrinters: []string{"192.168.1.100", "192.168.1.101"},
		},
	}

	printers := ListLANPrinters(cfg)
	testutil.ExpectedLen(t, printers, 2)
	testutil.ExpectedEqual(t, printers[0].IP, "192.168.1.100")
	testutil.ExpectedEqual(t, printers[0].Id, EncodeLANPrinterID("192.168.1.100"))
	testutil.ExpectedEqual(t, printers[1].IP, "192.168.1.101")
	testutil.ExpectedEqual(t, printers[1].Id, EncodeLANPrinterID("192.168.1.101"))
}
