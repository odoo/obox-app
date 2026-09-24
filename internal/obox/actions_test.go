package obox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"obox-app/internal/config"
	"obox-app/internal/testutil"
)

func TestObox_ActionPayload_PayloadBytes(t *testing.T) {
	// JSON string payload
	p1 := ActionPayload{Payload: json.RawMessage(`"<epos-print></epos-print>"`)}
	testutil.ExpectedEqual(t, string(p1.PayloadBytes()), "<epos-print></epos-print>")
	// Raw bytes payload
	p2 := ActionPayload{Payload: json.RawMessage(`<epos-print></epos-print>`)}
	testutil.ExpectedEqual(t, string(p2.PayloadBytes()), "<epos-print></epos-print>")
	// Null or empty
	p3 := ActionPayload{Payload: json.RawMessage(`null`)}
	testutil.ExpectedNil(t, p3.PayloadBytes())
	// Nil payload
	p4 := ActionPayload{Payload: nil}
	testutil.ExpectedNil(t, p4.PayloadBytes())
}

func TestObox_ExecuteAction_Delegation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	var received QueueAction
	handler := func(ctx context.Context, action QueueAction) (any, error) {
		received = action
		return map[string]string{"result": "ok"}, nil
	}

	m := NewManager(4545, cfg, handler)
	defer m.Stop()

	targetAction := QueueAction{
		UUID: "uuid-custom",
		Payload: ActionPayload{
			URL:    "/test/custom_action?debug=1",
			Method: "POST",
		},
	}
	m.executeAction(targetAction)

	testutil.ExpectedEqual(t, received.UUID, "uuid-custom")
	testutil.ExpectedEqual(t, received.Payload.URL, "/test/custom_action")
	testutil.ExpectedEqual(t, received.Payload.Method, "POST")
}

func TestObox_ExecuteAction_HandlerError(t *testing.T) {
	mockOdoo := testutil.NewMockOdooServer(t)

	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	handler := func(ctx context.Context, action QueueAction) (any, error) {
		return nil, errors.New("printer failure: connection refused")
	}

	m := NewManager(4545, cfg, handler)
	defer m.Stop()
	m.setCredentials(mockOdoo.URL, "tok", "uuid")

	m.executeAction(QueueAction{
		UUID:    "uuid-err-action",
		Payload: ActionPayload{URL: "/usb/v1/printer/p1/cgi-bin/epos/service.cgi"},
	})

	testutil.ExpectedEqual(t, <-mockOdoo.ActionReported, "uuid-err-action")
	resultStr := <-mockOdoo.LastResult

	var errMap map[string]string
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resultStr), &errMap))
	testutil.ExpectedEqual(t, errMap["error"], "printer failure: connection refused")
	testutil.ExpectedFalse(t, strings.Contains(resultStr, "not supported"))
}
