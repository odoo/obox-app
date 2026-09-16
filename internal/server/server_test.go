package server

import (
	"testing"

	"epos-proxy/internal/app_actions"
	"epos-proxy/internal/config"
	"epos-proxy/internal/obox"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"
)

// provides a centralized, clean test server setup for all server package tests.
func newTestServer(t testing.TB, cfg *config.Manager) (*Server, *printer.Manager) {
	t.Helper()
	if cfg == nil {
		t.Setenv("HOME", t.TempDir())
		var err error
		cfg, err = config.NewManager()
		testutil.ExpectedNoError(t, err)
	}
	if cfg.Data.Port == 0 {
		cfg.Data.Port = testutil.GetFreePort(t)
	}
	mgr := printer.NewManager(cfg)
	oboxMod := obox.NewManager(cfg.Data.Port, cfg, app_actions.OboxActionHandler(mgr))
	s := New(cfg.Data.Port, mgr, oboxMod)
	t.Cleanup(func() {
		_ = s.Stop()
		oboxMod.Stop()
	})
	return s, mgr
}

func TestServer_Lifecycle(t *testing.T) {
	s, _ := newTestServer(t, nil)
	testutil.ExpectedTrue(t, s.Running(), "Expected server to be running after New()")
	testutil.ExpectedTrue(t, s.Port > 0)

	err := s.Stop()
	testutil.ExpectedNoError(t, err)
}
