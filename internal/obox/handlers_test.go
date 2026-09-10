package obox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestObox_Routes(t *testing.T) {
	mockOdoo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": []interface{}{}})
	}))
	defer mockOdoo.Close()

	m, app := createTestModule(t)

	// 1. Initial /odoo/
	req := httptest.NewRequest("GET", "/odoo/", nil)
	resp, err := app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusServiceUnavailable)
	resp.Body.Close()

	// 2. /odoo/connect
	connectURL := "/odoo/connect?db_url=" + mockOdoo.URL + "&token=test-tok&db_uuid=test-uuid"
	req = httptest.NewRequest("GET", connectURL, nil)
	resp, err = app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)
	resp.Body.Close()

	// 3. /odoo/ configured discovery
	req = httptest.NewRequest("GET", "/odoo/", nil)
	resp, err = app.Test(req)
	testutil.ExpectedNoError(t, err)
	var discResp struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&discResp)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, discResp.Status, "configured")
	testutil.ExpectedEqual(t, discResp.Data["db_url"], mockOdoo.URL)
	resp.Body.Close()

	// 4. Disconnect — also stops the queue handler goroutine before the test exits.
	reqDisc := httptest.NewRequest("GET", "/odoo/disconnect", nil)
	respDisc, err := app.Test(reqDisc)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respDisc.StatusCode, http.StatusOK)
	respDisc.Body.Close()

	m.Stop()
}
