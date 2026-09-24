package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"obox-app/internal/config"
	"obox-app/internal/testutil"
)

func TestOboxRoutes_Lifecycle(t *testing.T) {
	mockOdoo := testutil.NewMockOdooServer(t)
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	s, _ := newTestServer(t, cfg)

	// 1. Initial discovery check (unconfigured)
	reqInit := httptest.NewRequest("GET", "/odoo/", nil)
	respInit, err := s.app.Test(reqInit)
	testutil.ExpectedNoError(t, err)
	defer respInit.Body.Close()
	testutil.ExpectedEqual(t, respInit.StatusCode, http.StatusServiceUnavailable)

	var initResult struct {
		Status string `json:"status"`
	}
	err = json.NewDecoder(respInit.Body).Decode(&initResult)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, initResult.Status, "not_configured")
	testutil.ExpectedFalse(t, cfg.HasOdooCredentials())

	// 2. Connect via /odoo/connect & verify persistence
	connectURL := "/odoo/connect?db_url=" + mockOdoo.URL + "&token=test-token-123&db_uuid=test-uuid"
	reqConnect := httptest.NewRequest("GET", connectURL, nil)
	respConnect, err := s.app.Test(reqConnect)
	testutil.ExpectedNoError(t, err)
	respConnect.Body.Close()
	testutil.ExpectedEqual(t, respConnect.StatusCode, http.StatusOK)
	testutil.ExpectedTrue(t, cfg.HasOdooCredentials())

	// 3. Configured discovery returns status and db_url
	reqStatus := httptest.NewRequest("GET", "/odoo/", nil)
	respStatus, err := s.app.Test(reqStatus)
	testutil.ExpectedNoError(t, err)
	defer respStatus.Body.Close()
	testutil.ExpectedEqual(t, respStatus.StatusCode, http.StatusOK)

	var statusResult struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}
	err = json.NewDecoder(respStatus.Body).Decode(&statusResult)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNotNil(t, statusResult.Data)
	testutil.ExpectedEqual(t, statusResult.Data["db_url"], mockOdoo.URL)

	// 4. Disconnect clears credentials
	reqDisc := httptest.NewRequest("GET", "/odoo/disconnect", nil)
	respDisc, err := s.app.Test(reqDisc)
	testutil.ExpectedNoError(t, err)
	respDisc.Body.Close()
	testutil.ExpectedEqual(t, respDisc.StatusCode, http.StatusOK)
	testutil.ExpectedFalse(t, cfg.HasOdooCredentials())
}

func TestOboxRestoreCredentialsFromConfig(t *testing.T) {
	mockOdoo := testutil.NewMockOdooServer(t)
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	testAppID := cfg.GetAppID()
	_ = cfg.SaveOdooCredentials(mockOdoo.URL, "restored-token", "uuid-1")

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

func TestOboxRoute_DirectPrint(t *testing.T) {
	s, _ := newTestServer(t, nil)

	req := httptest.NewRequest("POST", "/usb/v1/printer/any-id/cgi-bin/epos/service.cgi", bytes.NewReader([]byte("<invalid>xml</invalid>")))
	req.Header.Set("Content-Type", "text/xml")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedContains(t, string(body), `code="SchemaError"`)
}
