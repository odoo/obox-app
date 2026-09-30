package bluetooth

import (
	"runtime"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestSupportedTransportsByOS(t *testing.T) {
	transports := supportedTransportsByOS()
	testutil.ExpectedTrue(t, len(transports) > 0, "expected at least one transport returned by supportedTransportsByOS")

	if runtime.GOOS == "darwin" {
		testutil.ExpectedEqual(t, transports[0].name(), "BLE")
	} else {
		testutil.ExpectedEqual(t, transports[0].name(), "Classic")
	}
}
