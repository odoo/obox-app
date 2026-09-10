package obox

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

func TestObox_CallOdooPing(t *testing.T) {
	pingReceived := make(chan bool, 1)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/obox/ping" {
			pingReceived <- true
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	}))
	defer mockServer.Close()

	m, _ := createTestModule(t)
	m.SetCredentials(mockServer.URL, "valid-tok", "uuid")

	m.callOdooPing()

	testutil.ExpectedTrue(t, <-pingReceived)
}

func TestObox_CallOdooOboxConnect(t *testing.T) {
	connectReceived := make(chan bool, 1)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/obox/connect":
			var req struct {
				Params struct {
					SerialNumber string   `json:"serial_number"`
					Token        string   `json:"token"`
					LocalIP      string   `json:"local_ip"`
					Services     []string `json:"services"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Params.Token == "conn-tok" {
				connectReceived <- true
				raw := json.RawMessage(`{"status": "paired"}`)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": &raw})
				return
			}
		case "/obox/get_next_actions":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": []interface{}{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"code": 404, "message": "not found"},
		})
	}))
	defer mockServer.Close()

	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	m := NewManager(4545, cfg, nil)
	t.Cleanup(m.Stop)
	m.SetCredentials(mockServer.URL, "conn-tok", "conn-uuid")
	m.callOdooOboxConnect(mockServer.URL, "conn-tok")

	testutil.ExpectedTrue(t, <-connectReceived)
	dbURL, tok := m.GetCredentials()
	testutil.ExpectedEqual(t, dbURL, mockServer.URL)
	testutil.ExpectedEqual(t, tok, "conn-tok")
	testutil.ExpectedEqual(t, m.GetWebsocketStatus(), StatusConnected)
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
