package main

import (
	"context"
	"testing"
	"time"

	"obox-app/internal/config"
	"obox-app/internal/testutil"
)

type mockEmitter struct {
	emittedEvents []string
}

func (m *mockEmitter) Emit(ctx context.Context, eventName string, optionalData ...interface{}) {
	m.emittedEvents = append(m.emittedEvents, eventName)
}

func TestApp_OdooSessions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	emitter := &mockEmitter{}
	app := &App{
		config: cfg,
		events: emitter,
	}

	// 1. Initial instances should be empty
	instances := app.GetOdooInstances()
	testutil.ExpectedEqual(t, len(instances), 0)

	// 2. Add an instance
	inst1, err := app.AddOdooInstance("https://demo.odoo.com", "Demo Instance")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, inst1.URL, "https://demo.odoo.com")
	testutil.ExpectedEqual(t, inst1.Name, "Demo Instance")

	// 3. Prevent duplicate
	_, err = app.AddOdooInstance("https://demo.odoo.com/web", "Duplicate")
	testutil.ExpectedTrue(t, err != nil)

	// 4. Open session generates nonce and pending session
	err = app.OpenOdooSession(inst1.ID)
	testutil.ExpectedNoError(t, err)

	app.sessionMu.RLock()
	pending := app.pendingSession
	app.sessionMu.RUnlock()
	testutil.ExpectedTrue(t, pending != nil)
	testutil.ExpectedEqual(t, pending.InstanceID, inst1.ID)
	testutil.ExpectedTrue(t, pending.Nonce != "")

	// 5. Complete pairing with wrong nonce fails
	err = app.CompleteOdooSessionPairing("wrong-nonce", inst1.ID, "tok-123", "https://demo.odoo.com", 10)
	testutil.ExpectedTrue(t, err != nil)

	// 6. Complete pairing with correct nonce succeeds
	correctNonce := pending.Nonce
	err = app.CompleteOdooSessionPairing(correctNonce, inst1.ID, "tok-123", "https://demo.odoo.com", 10)
	testutil.ExpectedNoError(t, err)

	// 6b. Idempotent completion: calling again with the same nonce succeeds without error
	err = app.CompleteOdooSessionPairing(correctNonce, inst1.ID, "tok-123", "https://demo.odoo.com", 10)
	testutil.ExpectedNoError(t, err)

	active := app.config.GetActiveOdooInstance()
	testutil.ExpectedTrue(t, active != nil)
	testutil.ExpectedEqual(t, active.ID, inst1.ID)
	testutil.ExpectedEqual(t, active.Token, "tok-123")
	testutil.ExpectedEqual(t, active.OboxID, 10)

	// 7. Nonce is cleared after completion
	app.sessionMu.RLock()
	pendingAfter := app.pendingSession
	app.sessionMu.RUnlock()
	testutil.ExpectedTrue(t, pendingAfter == nil)

	// 8. Add second instance and switch
	inst2, err := app.AddOdooInstance("https://staging.odoo.com", "Staging")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, len(app.GetOdooInstances()), 2)

	err = app.SetActiveOdooInstance(inst2.ID)
	testutil.ExpectedNoError(t, err)
	active2 := app.config.GetActiveOdooInstance()
	testutil.ExpectedTrue(t, active2 != nil)
	testutil.ExpectedEqual(t, active2.ID, inst2.ID)

	// 9. Remove instance
	err = app.RemoveOdooInstance(inst1.ID)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, len(app.GetOdooInstances()), 1)
}

func TestApp_CompleteOdooSessionPairing_Expired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	app := &App{
		config: cfg,
		events: &mockEmitter{},
	}

	inst, err := app.AddOdooInstance("https://expired.odoo.com", "Expired")
	testutil.ExpectedNoError(t, err)

	app.sessionMu.Lock()
	app.pendingSession = &pendingOdooSession{
		InstanceID:  inst.ID,
		InstanceURL: inst.URL,
		Nonce:       "nonce-old",
		ExpiresAt:   time.Now().Add(-1 * time.Minute), // expired
	}
	app.sessionMu.Unlock()

	err = app.CompleteOdooSessionPairing("nonce-old", inst.ID, "tok-old", "https://expired.odoo.com", 1)
	testutil.ExpectedTrue(t, err != nil)
}

func TestApp_SyncOdooInstance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	app := &App{
		config: cfg,
		events: &mockEmitter{},
	}

	// 1. Sync non-existent instance fails
	err = app.SyncOdooInstance("non-existent")
	testutil.ExpectedTrue(t, err != nil)

	// 2. Sync unpaired instance fails
	inst, err := app.AddOdooInstance("https://unpaired.odoo.com", "Unpaired")
	testutil.ExpectedNoError(t, err)

	err = app.SyncOdooInstance(inst.ID)
	testutil.ExpectedTrue(t, err != nil)
}
