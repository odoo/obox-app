package obox

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"obox-app/internal/config"
	"obox-app/internal/testutil"

	"github.com/gofiber/fiber/v3"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	m := NewManager(4545, cfg, nil)
	t.Cleanup(m.Stop)
	return m
}

func createTestApp(m *Manager) *fiber.App {
	app := fiber.New()
	app.Get("/odoo/", m.HandleLanConnection)
	app.Get("/odoo/connect", m.HandleOfflineConnect)
	app.Get("/odoo/disconnect", m.HandleDisconnect)
	return app
}

func createTestModule(t *testing.T) (*Manager, *fiber.App) {
	t.Helper()
	m := newTestManager(t)
	return m, createTestApp(m)
}

func TestObox_CredentialsAndConnection(t *testing.T) {
	m := newTestManager(t)
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateDisconnected)

	m.setCredentials("http://127.0.0.1:8069", "token-xyz", "db-uuid-1")

	dbURL, tok := m.getCredentials()
	testutil.ExpectedEqual(t, dbURL, "http://127.0.0.1:8069")
	testutil.ExpectedEqual(t, tok, "token-xyz")
	testutil.ExpectedEqual(t, m.getDbURL(), "http://127.0.0.1:8069")
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateDisconnected)

	m.Disconnect()
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateDisconnected)
}

func TestObox_LANContact(t *testing.T) {
	m := newTestManager(t)
	app := createTestApp(m)
	testutil.ExpectedEqual(t, m.getLANStatus(), StateDisconnected)
	m.setCredentials("http://127.0.0.1:8069", "tok", "")

	statusCh := make(chan ConnectionStatus, 5)
	m.SetOnStatusChange(func(status ConnectionStatus) {
		statusCh <- status
	})

	// 1. Initial contact via LAN route -> transitions to connected
	req, _ := http.NewRequest("GET", "/odoo/", nil)
	_, err := app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, m.getLANStatus(), StateConnected)

	select {
	case status := <-statusCh:
		testutil.ExpectedEqual(t, status.LanStatus, StateConnected)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for status change")
	}

	for len(statusCh) > 0 {
		<-statusCh
	}

	// 2. Duplicate contact -> stays connected, listener not re-triggered
	req, _ = http.NewRequest("GET", "/odoo/", nil)
	_, err = app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, m.getLANStatus(), StateConnected)
	select {
	case <-statusCh:
		t.Fatal("status should not have changed")
	case <-time.After(50 * time.Millisecond):
	}

	// 3. Timeout elapsed (>30s) -> getLANStatus derives disconnected
	m.lastLANContact.Store(time.Now().Add(-31 * time.Second).UnixMilli())
	testutil.ExpectedEqual(t, m.getLANStatus(), StateDisconnected)

	// 4. Explicit disconnect
	m.Disconnect()
	testutil.ExpectedEqual(t, m.getLANStatus(), StateDisconnected)
}

func TestObox_ConnectStateMachine_Success(t *testing.T) {
	mockServer := testutil.NewMockOdooServer(t)

	m := newTestManager(t)
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateDisconnected)

	m.connect(mockServer.URL, "tok-1", "uuid-1")

	select {
	case tok := <-mockServer.ConnectTokens:
		testutil.ExpectedEqual(t, tok, "tok-1")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for /obox/connect")
	}

	select {
	case <-mockServer.PollCount:
		// Queue handler started polling after connection succeeded
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queue handler to poll /obox/get_next_actions")
	}

	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateConnected)
	dbURL, tok := m.getCredentials()
	testutil.ExpectedEqual(t, dbURL, mockServer.URL)
	testutil.ExpectedEqual(t, tok, "tok-1")
}

func TestObox_ConnectStateMachine_DisconnectCancels(t *testing.T) {
	connectStarted := make(chan struct{})

	mockServer := testutil.NewMockOdooServer(t)
	mockServer.CustomHandler = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/obox/connect" {
			close(connectStarted)
			select {
			case <-r.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
			return true
		}
		return false
	}

	m := newTestManager(t)
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateDisconnected)

	m.connect(mockServer.URL, "tok-hang", "uuid-hang")
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateConnecting)

	<-connectStarted
	m.Disconnect()
	testutil.ExpectedEqual(t, m.getWebsocketStatus(), StateDisconnected)
}

func TestObox_ConnectStateMachine_ReconnectDoesNotCancelNewAttempt(t *testing.T) {
	attempt1Started := make(chan struct{})
	attempt2Connected := make(chan struct{})

	mockServer := testutil.NewMockOdooServer(t)
	mockServer.CustomHandler = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/obox/connect" {
			var req struct {
				Params struct {
					Token string `json:"token"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Params.Token == "tok-1" {
				close(attempt1Started)
				// Hang until cancelled by reconnect
				select {
				case <-r.Context().Done():
				case <-time.After(2 * time.Second):
				}
				return true
			}
			if req.Params.Token == "tok-2" {
				raw := json.RawMessage(`{"status": "paired"}`)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": &raw})
				select {
				case <-attempt2Connected:
				default:
					close(attempt2Connected)
				}
				return true
			}
		}
		return false
	}

	m := newTestManager(t)

	// Start attempt 1
	m.connect(mockServer.URL, "tok-1", "uuid-1")
	<-attempt1Started

	// Trigger attempt 2 while attempt 1 is still unwinding
	m.connect(mockServer.URL, "tok-2", "uuid-2")

	select {
	case <-attempt2Connected:
		// Succeeded! Attempt 2 reached the server
	case <-time.After(2 * time.Second):
		t.Fatal("attempt 2 did not connect in time (possibly cancelled by attempt 1 defer)")
	}

	connected := false
	for i := 0; i < 50; i++ {
		if m.getWebsocketStatus() == StateConnected {
			connected = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	testutil.ExpectedTrue(t, connected, "expected websocket status to be connected")
}

func TestObox_Status_InOrderRapidTransitions(t *testing.T) {
	m := newTestManager(t)

	var received []ConnectionState
	m.SetOnStatusChange(func(status ConnectionStatus) {
		received = append(received, status.WebsocketStatus)
	})

	m.setWsStatus(StateConnecting)
	m.setWsStatus(StateConnected)
	m.setWsStatus(StateDisconnected)

	testutil.ExpectedEqual(t, len(received), 3)
	testutil.ExpectedEqual(t, received[0], StateConnecting)
	testutil.ExpectedEqual(t, received[1], StateConnected)
	testutil.ExpectedEqual(t, received[2], StateDisconnected)
}

func TestObox_StatusChange_ReentrantSafe(t *testing.T) {
	m := newTestManager(t)

	done := make(chan struct{})
	m.SetOnStatusChange(func(status ConnectionStatus) {
		// Re-enter the manager while in status callback
		_ = m.ConnectionStatus()
		m.Disconnect()
		m.SetOnStatusChange(nil)
		select {
		case <-done:
		default:
			close(done)
		}
	})

	m.setLANStatus(StateConnected)

	select {
	case <-done:
		// Succeeded without deadlock
	case <-time.After(1 * time.Second):
		t.Fatal("deadlock occurred when re-entering manager from onStatusChange callback")
	}
}
