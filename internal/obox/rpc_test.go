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
	m.setCredentials(mockServer.URL, "valid-tok", "uuid")

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
	m.setCredentials(mockServer.URL, "conn-tok", "conn-uuid")
	_ = m.callOdooOboxConnect(context.Background(), mockServer.URL, "conn-tok")

	testutil.ExpectedEqual(t, <-mockServer.ConnectTokens, "conn-tok")
	dbURL, tok := m.getCredentials()
	testutil.ExpectedEqual(t, dbURL, mockServer.URL)
	testutil.ExpectedEqual(t, tok, "conn-tok")
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateConnected)
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
