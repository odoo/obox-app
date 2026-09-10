package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

func TestOboxDiscovery_Endpoint(t *testing.T) {
	s, _ := newTestServer(t, nil)

	req := httptest.NewRequest("GET", "/odoo/", nil)
	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	defer resp.Body.Close()

	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusServiceUnavailable)

	var result struct {
		Status string `json:"status"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, result.Status, "not_configured")
}

func TestOboxConnectDB_Endpoint(t *testing.T) {
	mockOdoo := newMockOdoo(t)
	s, _ := newTestServer(t, nil)

	connectURL := "/odoo/connect?db_url=" + mockOdoo.URL + "&token=test-token-123&db_uuid=test-uuid"
	req := httptest.NewRequest("GET", connectURL, nil)
	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	resp.Body.Close()
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)

	reqStatus := httptest.NewRequest("GET", "/odoo/", nil)
	respStatus, err := s.app.Test(reqStatus)
	testutil.ExpectedNoError(t, err)
	defer respStatus.Body.Close()

	var result struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}
	err = json.NewDecoder(respStatus.Body).Decode(&result)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, result.Data)
	testutil.ExpectedEqual(t, result.Data["db_url"], mockOdoo.URL)

	reqDisc := httptest.NewRequest("GET", "/odoo/disconnect", nil)
	respDisc, err := s.app.Test(reqDisc)
	testutil.ExpectedNoError(t, err)
	respDisc.Body.Close()
}

func TestOboxStoragePersistence(t *testing.T) {
	mockOdoo := newMockOdoo(t)
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	s, _ := newTestServer(t, cfg)
	// 1. Initially no credentials
	testutil.ExpectedFalse(t, cfg.HasOdooCredentials())

	// 2. Connect persists credentials
	connectURL := "/odoo/connect?db_url=" + mockOdoo.URL + "&db_uuid=test-uuid-456&token=test-tok-789"
	req := httptest.NewRequest("GET", connectURL, nil)
	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	resp.Body.Close()
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)

	testutil.ExpectedTrue(t, cfg.HasOdooCredentials())
	testutil.ExpectedEqual(t, cfg.GetOdooDbURL(), mockOdoo.URL)

	// 3. Disconnect clears credentials
	reqDisc := httptest.NewRequest("GET", "/odoo/disconnect", nil)
	respDisc, err := s.app.Test(reqDisc)
	testutil.ExpectedNoError(t, err)
	respDisc.Body.Close()

	testutil.ExpectedFalse(t, cfg.HasOdooCredentials())
}

func TestOboxRestoreCredentialsFromConfig(t *testing.T) {
	mockOdoo := newMockOdoo(t)
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	testAppID := cfg.GetAppID()
	_ = cfg.SetOdooCredentials(mockOdoo.URL, "restored-token", "uuid-1")

	s, _ := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/odoo/", nil)
	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	defer resp.Body.Close()

	var result struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, result.Data)
	testutil.ExpectedEqual(t, result.Data["serial"], testAppID)
	testutil.ExpectedEqual(t, result.Data["db_url"], mockOdoo.URL)

	reqDisc := httptest.NewRequest("GET", "/odoo/disconnect", nil)
	respDisc, err := s.app.Test(reqDisc)
	testutil.ExpectedNoError(t, err)
	respDisc.Body.Close()
}

func TestGetOdooStatus(t *testing.T) {
	mockOdoo := newMockOdoo(t)
	s, _ := newTestServer(t, nil)

	// 1. Disconnected
	testutil.ExpectedEqual(t, s.obox.GetWebsocketStatus(), "disconnected")

	// 2. Connect
	connectURL := "/odoo/connect?db_url=" + mockOdoo.URL + "&token=status-token-123&db_uuid=test-uuid"
	reqConnect := httptest.NewRequest("GET", connectURL, nil)
	respConnect, err := s.app.Test(reqConnect)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respConnect.StatusCode, http.StatusOK)
	respConnect.Body.Close()

	// 3. Verify db_url
	testutil.ExpectedEqual(t, s.obox.GetDbURL(), mockOdoo.URL)
	for range 50 {
		if s.obox.GetWebsocketStatus() == "connected" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 4. Disconnect
	reqDisc := httptest.NewRequest("GET", "/odoo/disconnect", nil)
	respDisc, err := s.app.Test(reqDisc)
	testutil.ExpectedNoError(t, err)
	respDisc.Body.Close()

	testutil.ExpectedEqual(t, s.obox.GetWebsocketStatus(), "disconnected")
}
