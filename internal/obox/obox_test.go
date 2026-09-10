package obox

import (
	"net/http"
	"testing"
	"time"

	"epos-proxy/internal/config"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"

	"github.com/gofiber/fiber/v3"
)

func createTestModule(t *testing.T) (*Manager, *fiber.App) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	app := fiber.New()

	pm := printer.NewManager(4545, cfg)
	m := NewManager(4545, cfg, pm)
	t.Cleanup(m.Stop)
	app.Get("/odoo/", m.HandleLanConnection)
	app.Get("/odoo/connect", m.HandleOfflineConnect)
	app.Get("/odoo/disconnect", m.HandleDisconnect)
	return m, app
}

func TestObox_CredentialsAndConnection(t *testing.T) {
	m, _ := createTestModule(t)
	testutil.ExpectedEqual(t, m.GetWebsocketStatus(), "disconnected")

	m.SetCredentials("http://127.0.0.1:8069", "token-xyz", "db-uuid-1")

	dbURL, tok := m.GetCredentials()
	testutil.ExpectedEqual(t, dbURL, "http://127.0.0.1:8069")
	testutil.ExpectedEqual(t, tok, "token-xyz")
	testutil.ExpectedEqual(t, m.GetDbURL(), "http://127.0.0.1:8069")

	m.ClearCredentials()
	testutil.ExpectedEqual(t, m.GetDbURL(), "")
}

func TestObox_StatusChangeListener(t *testing.T) {
	m, _ := createTestModule(t)

	called := make(chan ConnectionStatus, 1)
	m.SetOnStatusChange(func(status ConnectionStatus) {
		called <- status
	})

	m.setWsStatus(StatusConnected)
	select {
	case status := <-called:
		testutil.ExpectedEqual(t, status.WebsocketStatus, StatusConnected)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for status change")
	}
}

func TestObox_LANContact(t *testing.T) {
	m, app := createTestModule(t)
	testutil.ExpectedEqual(t, m.GetLANStatus(), StatusDisconnected)
	m.setCredentials("http://127.0.0.1:8069", "tok")

	statusCh := make(chan ConnectionStatus, 5)
	m.SetOnStatusChange(func(status ConnectionStatus) {
		statusCh <- status
	})

	req, _ := http.NewRequest("GET", "/odoo/", nil)
	_, err := app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, m.GetLANStatus(), StatusConnected)

	select {
	case status := <-statusCh:
		testutil.ExpectedEqual(t, status.LanStatus, StatusConnected)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for status change")
	}

	for len(statusCh) > 0 {
		<-statusCh
	}

	req, _ = http.NewRequest("GET", "/odoo/", nil)
	_, err = app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, m.GetLANStatus(), "connected")
	select {
	case <-statusCh:
		t.Fatal("status should not have changed")
	case <-time.After(50 * time.Millisecond):
	}

	m.Disconnect()
	testutil.ExpectedEqual(t, m.GetLANStatus(), StatusDisconnected)
}

func TestObox_LANTimeoutDirectDisconnect(t *testing.T) {
	m, _ := createTestModule(t)
	m.setCredentials("http://127.0.0.1:8069", "tok")

	m.RecordLANContact()
	testutil.ExpectedEqual(t, m.GetLANStatus(), StatusConnected)
	testutil.ExpectedTrue(t, !m.LastLANContact().IsZero())

	// Artificially simulate 31 seconds having passed since last contact
	m.lastLANContact.Store(time.Now().Add(-31 * time.Second).UnixMilli())

	// GetLANStatus immediately derives disconnected because timeout elapsed
	testutil.ExpectedEqual(t, m.GetLANStatus(), StatusDisconnected)

	// Trigger the timer manually or verify setLANStatus transitions directly to disconnected
	statusCh := make(chan ConnectionStatus, 5)
	m.SetOnStatusChange(func(status ConnectionStatus) {
		statusCh <- status
	})

	m.lanMu.Lock()
	if m.lanTimer != nil {
		m.lanTimer.Stop()
	}
	m.lanMu.Unlock()

	// Calling setLANStatus with disconnected directly
	m.setLANStatus(StatusDisconnected)
	testutil.ExpectedEqual(t, m.GetLANStatus(), StatusDisconnected)

	select {
	case status := <-statusCh:
		testutil.ExpectedEqual(t, status.LanStatus, StatusDisconnected)
	case <-time.After(50 * time.Millisecond):
	}
}
