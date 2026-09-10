package printer

import (
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

func TestManager_PrintReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	mgr := NewManager(4545, cfg)

	// 1. Invalid XML SchemaError
	resp := mgr.PrintReceipt("any-printer", []byte("<invalid>xml</invalid>"))
	testutil.ExpectedFalse(t, resp.Success)
	testutil.ExpectedEqual(t, resp.Code, "SchemaError")

	// 2. Unreachable printer EX_BADPORT
	resp = mgr.PrintReceipt("czpOT05fRVhJU1RFTlRfU0VSSUFMCg", []byte("<epos-print><text>hello</text></epos-print>"))
	testutil.ExpectedFalse(t, resp.Success)
	testutil.ExpectedEqual(t, resp.Code, "EX_BADPORT")

	// 3. Nil manager check
	var nilMgr *Manager
	resp = nilMgr.PrintReceipt("any-printer", []byte("<epos-print><text>hello</text></epos-print>"))
	testutil.ExpectedFalse(t, resp.Success)
	testutil.ExpectedEqual(t, resp.Code, "EX_BADPORT")
}
