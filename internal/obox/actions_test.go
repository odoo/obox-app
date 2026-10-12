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
	m.setCredentials(mockOdoo.URL, "tok")

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

func TestObox_ExecuteAction_OpenKiosk(t *testing.T) {
	mockOdoo := testutil.NewMockOdooServer(t)

	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	m := NewManager(4545, cfg, nil)
	defer m.Stop()
	m.setCredentials(mockOdoo.URL, "tok")

	var openedURL string
	openCh := make(chan string, 1)
	m.SetOnOpenKiosk(func(url string) {
		openCh <- url
	})

	kioskPayload := `{"url": "https://example.com/pos-self/204?access_token=tok123", "timezone": "Europe/Brussels"}`
	m.executeAction(QueueAction{
		UUID: "uuid-open-kiosk",
		Payload: ActionPayload{
			URL:     "/odoo/open_kiosk",
			Method:  "GET",
			Payload: json.RawMessage(kioskPayload),
		},
	})

	testutil.ExpectedEqual(t, <-mockOdoo.ActionReported, "uuid-open-kiosk")
	resultStr := <-mockOdoo.LastResult

	var resMap map[string]string
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resultStr), &resMap))
	testutil.ExpectedEqual(t, resMap["status"], "opened")

	openedURL = <-openCh
	testutil.ExpectedEqual(t, openedURL, "https://example.com/pos-self/204?access_token=tok123")
	testutil.ExpectedEqual(t, cfg.GetWebViewURL(), "https://example.com/pos-self/204?access_token=tok123")
	testutil.ExpectedTrue(t, cfg.GetWebViewEnabled())
}

func TestObox_ExecuteAction_OpenKiosk_InvalidURL(t *testing.T) {
	mockOdoo := testutil.NewMockOdooServer(t)

	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	m := NewManager(4545, cfg, nil)
	defer m.Stop()
	m.setCredentials(mockOdoo.URL, "tok")

	m.executeAction(QueueAction{
		UUID: "uuid-invalid-kiosk",
		Payload: ActionPayload{
			URL:     "/odoo/open_kiosk",
			Method:  "GET",
			Payload: json.RawMessage(`{"url": "not-a-valid-url"}`),
		},
	})

	testutil.ExpectedEqual(t, <-mockOdoo.ActionReported, "uuid-invalid-kiosk")
	resultStr := <-mockOdoo.LastResult

	var resMap map[string]string
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resultStr), &resMap))
	testutil.ExpectedTrue(t, strings.Contains(resMap["error"], "invalid URL"))
}

