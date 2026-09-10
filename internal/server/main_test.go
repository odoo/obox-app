package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
	mgr := printer.NewManager(cfg.Data.Port, cfg)
	oboxMod := obox.NewManager(cfg.Data.Port, cfg, mgr)
	s := New(cfg.Data.Port, mgr, oboxMod)
	t.Cleanup(func() {
		_ = s.Stop()
		oboxMod.Stop()
	})
	return s, mgr
}

// starts an httptest.Server that mocks Odoo obox JSON-RPC endpoints.
func newMockOdoo(t testing.TB) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/obox/connect":
			raw := json.RawMessage(`{"status":"paired"}`)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": &raw})
		case "/obox/get_next_actions":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": []interface{}{}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestServer_Lifecycle(t *testing.T) {
	s, _ := newTestServer(t, nil)
	testutil.ExpectedTrue(t, s.Running(), "Expected server to be running after New()")
	testutil.ExpectedTrue(t, s.Port > 0)
}

func TestCustomSerialNumberSupport(t *testing.T) {
	mockOdoo := newMockOdoo(t)
	cfg := &config.Manager{Data: config.AppConfig{AppID: "CUSTOM_SERIAL_123"}}
	s, _ := newTestServer(t, cfg)

	// 1. Initial /odoo/ check (unconfigured)
	reqInit := httptest.NewRequest("GET", "/odoo/", nil)
	respInit, err := s.app.Test(reqInit)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respInit.StatusCode, http.StatusServiceUnavailable)
	respInit.Body.Close()

	// 2. Connect via /odoo/connect
	reqConnect := httptest.NewRequest("GET", "/odoo/connect?db_url="+mockOdoo.URL+"&token=test-token&db_uuid=test-uuid", nil)
	respConnect, err := s.app.Test(reqConnect)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respConnect.StatusCode, http.StatusOK)
	respConnect.Body.Close()

	// 3. Verify /odoo/ returns custom serial
	reqStatus := httptest.NewRequest("GET", "/odoo/", nil)
	respStatus, err := s.app.Test(reqStatus)
	testutil.ExpectedNoError(t, err)
	defer respStatus.Body.Close()
	bodyBytes, _ := io.ReadAll(respStatus.Body)
	testutil.ExpectedContains(t, string(bodyBytes), "CUSTOM_SERIAL_123")
}
