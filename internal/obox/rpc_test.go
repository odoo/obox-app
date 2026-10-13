package obox

import (
	"context"
	"errors"
	"testing"

	"obox-app/internal/config"
	"obox-app/internal/testutil"
)

func TestObox_CallOdooPing(t *testing.T) {
	mockServer := testutil.NewMockOdooServer(t)

	m, _ := createTestModule(t)
	m.setCredentials(mockServer.URL, "valid-tok")

	m.callOdooPing()

	testutil.ExpectedTrue(t, <-mockServer.PingReceived)
}

func TestObox_CallOdooOboxConnect(t *testing.T) {
	mockServer := testutil.NewMockOdooServer(t)

	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	m := NewManager(4545, cfg, nil)
	t.Cleanup(m.Stop)
	m.setCredentials(mockServer.URL, "conn-tok")
	_ = m.callOdooOboxConnect(context.Background(), mockServer.URL, "conn-tok")

	testutil.ExpectedEqual(t, <-mockServer.ConnectTokens, "conn-tok")
	dbURL, tok := m.getCredentials()
	testutil.ExpectedEqual(t, dbURL, mockServer.URL)
	testutil.ExpectedEqual(t, tok, "conn-tok")
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateConnected)

	// Verify services includes "sync" and "kiosk"
	params := <-mockServer.ConnectParamsReceived
	services, ok := params["services"].([]interface{})
	testutil.ExpectedTrue(t, ok)
	hasSync := false
	hasKiosk := false
	for _, s := range services {
		if s == "sync" {
			hasSync = true
		}
		if s == "kiosk" {
			hasKiosk = true
		}
	}
	testutil.ExpectedTrue(t, hasSync)
	testutil.ExpectedTrue(t, hasKiosk)
}

func TestObox_BuildDeviceState_AllFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	m := NewManager(4545, cfg, nil)
	t.Cleanup(m.Stop)

	m.SetDeviceProvider(func() []map[string]interface{} {
		return []map[string]interface{}{
			{"identifier": "printer-1", "name": "Epson TM-T88VI", "type": "printer"},
		}
	})
	m.SetAppliedConfigVersion("hash123")

	state := m.BuildDeviceState()

	// Top-level fields
	testutil.ExpectedEqual(t, state["name"], "OBOX App")
	testutil.ExpectedEqual(t, state["locale"], "en-US")
	testutil.ExpectedEqual(t, state["is_device_owner"], true)
	testutil.ExpectedEqual(t, state["config_version"], "hash123")
	testutil.ExpectedEqual(t, state["local_port"], 4545)

	// Hardware fields (matching _compute_android_info)
	hw, ok := state["hardware"].(map[string]interface{})
	testutil.ExpectedEqual(t, hw["android_id"].(string), cfg.GetAppID())
	testutil.ExpectedEqual(t, hw["version"].(string), "2.15.0")
	testutil.ExpectedNotEqual(t, hw["model"], "")
	testutil.ExpectedNotEqual(t, hw["manufacturer"], "")
	testutil.ExpectedNotEqual(t, hw["processor"], "")
	testutil.ExpectedNotEqual(t, hw["android_version"], "")
	testutil.ExpectedNotEqual(t, hw["api_level"], "")
	testutil.ExpectedNotEqual(t, hw["android_build"], "")
	testutil.ExpectedNotEqual(t, hw["resolution"], "")

	webview, ok := hw["webview"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedNotEqual(t, webview["provider"], "")
	testutil.ExpectedNotEqual(t, webview["version"], "")
	testutil.ExpectedNotEqual(t, webview["ua"], "")

	// Metrics fields
	metrics, ok := state["metrics"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, metrics["screen_on"], true)
	testutil.ExpectedEqual(t, metrics["signal_level"], 4)
	testutil.ExpectedEqual(t, metrics["brightness"], 100)
	testutil.ExpectedEqual(t, metrics["volume"], 80)
	testutil.ExpectedTrue(t, metrics["up_since"].(int64) > 0)

	battery, ok := metrics["battery"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, battery["level"], 100)
	testutil.ExpectedEqual(t, battery["charging"], true)
	testutil.ExpectedEqual(t, battery["plugged"], true)

	mem, ok := metrics["memory"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedTrue(t, mem["used"].(int64) > 0)
	testutil.ExpectedTrue(t, mem["total"].(int64) > 0)

	storage, ok := metrics["storage"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedTrue(t, storage["used"].(int64) > 0)
	testutil.ExpectedTrue(t, storage["total"].(int64) > 0)

	// Network fields
	net, ok := state["network"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, net["ethernet"], true)
	testutil.ExpectedEqual(t, net["wifi"], false)

	// Kiosk fields
	kiosk, ok := state["kiosk"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, kiosk["kiosk_pin"], "0000")
	testutil.ExpectedEqual(t, kiosk["out_of_service"], false)

	// Devices fields
	devs, ok := state["devices"].(map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	added, ok := devs["added"].([]map[string]interface{})
	testutil.ExpectedTrue(t, ok)
	testutil.ExpectedEqual(t, len(added), 1)
	testutil.ExpectedEqual(t, added[0]["identifier"], "printer-1")
}

func TestObox_Sync_ConfigResponse_AppliesPIN(t *testing.T) {
	mockServer := testutil.NewMockOdooServer(t)

	// Set mock Odoo to return a new PIN from Odoo config
	mockServer.SetConfigResponse(map[string]interface{}{
		"config_up_to_date": false,
		"config_version":    "hash-new-version-999",
		"config": map[string]interface{}{
			"name":      "Updated Obox",
			"kiosk_pin": "5678",
			"features":  []interface{}{},
			"devices":   []interface{}{},
		},
	})

	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	m := NewManager(4545, cfg, nil)
	t.Cleanup(m.Stop)
	m.setCredentials(mockServer.URL, "tok-sync")

	testutil.ExpectedEqual(t, cfg.GetWebViewPIN(), "0000")

	err = m.Sync(context.Background())
	testutil.ExpectedNoError(t, err)

	// Verify transmitted state
	state := <-mockServer.SyncReceived
	testutil.ExpectedNotNil(t, state)

	// Verify that PIN and config version were updated
	testutil.ExpectedEqual(t, cfg.GetWebViewPIN(), "5678")
	testutil.ExpectedEqual(t, m.GetAppliedConfigVersion(), "hash-new-version-999")
}

func TestObox_IsDeviceNotFound(t *testing.T) {
	// 1. Non-rpcError
	testutil.ExpectedFalse(t, isDeviceNotFound(errors.New("regular network error")))
	testutil.ExpectedFalse(t, isDeviceNotFound(nil))

	// 2. rpcError with 404 code
	err404 := &rpcError{Code: 404, Message: "404: Not Found"}
	testutil.ExpectedTrue(t, isDeviceNotFound(err404))
	testutil.ExpectedContains(t, err404.Error(), "404")

	// 3. rpcError with werkzeug NotFound exception name
	errWerkzeug := &rpcError{
		Code:    200,
		Message: "Odoo Error",
		Data: struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		}{
			Name:    "werkzeug.exceptions.NotFound",
			Message: "404 Not Found",
		},
	}
	testutil.ExpectedTrue(t, isDeviceNotFound(errWerkzeug))
	testutil.ExpectedContains(t, errWerkzeug.Error(), "werkzeug.exceptions.NotFound")

	// 4. Other RPC error (e.g. 500 Internal Server Error or AccessDenied)
	err500 := &rpcError{Code: 500, Message: "Internal Server Error"}
	testutil.ExpectedFalse(t, isDeviceNotFound(err500))
}
