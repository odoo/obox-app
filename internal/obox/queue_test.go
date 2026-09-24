package obox

import (
	"encoding/json"
	"net/http"
	"testing"

	"obox-app/internal/testutil"
)

func TestObox_executeAction(t *testing.T) {
	mockOdoo := testutil.NewMockOdooServer(t)

	m, _ := createTestModule(t)
	m.setCredentials(mockOdoo.URL, "tok", "uuid")

	// 1. Health ping action
	actionHealth := QueueAction{
		UUID: "action-health",
		Payload: ActionPayload{
			URL:    "/odoo/health",
			Method: "GET",
		},
	}
	m.executeAction(actionHealth)
	testutil.ExpectedEqual(t, <-mockOdoo.ActionReported, "action-health")
	resHealth := <-mockOdoo.LastResult
	var healthMap map[string]interface{}
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resHealth), &healthMap))
	testutil.ExpectedEqual(t, healthMap["status"], "ok")

	// 2. Restart action
	actionRestart := QueueAction{
		UUID: "action-restart",
		Payload: ActionPayload{
			URL:    "/odoo/restart",
			Method: "GET",
		},
	}
	m.executeAction(actionRestart)
	testutil.ExpectedEqual(t, <-mockOdoo.ActionReported, "action-restart")
	resRestart := <-mockOdoo.LastResult
	var restartMap map[string]interface{}
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resRestart), &restartMap))
	testutil.ExpectedEqual(t, restartMap["error"], "Restart is not supported on Obox app")

	// 3. Unsupported action fallback (no handler match)
	actionUnsupported := QueueAction{
		UUID: "action-unsupported",
		Payload: ActionPayload{
			URL:    "/unknown/action",
			Method: "GET",
		},
	}
	m.executeAction(actionUnsupported)
	testutil.ExpectedEqual(t, <-mockOdoo.ActionReported, "action-unsupported")
	resUnsupported := <-mockOdoo.LastResult
	var unsupportedMap map[string]interface{}
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resUnsupported), &unsupportedMap))
	testutil.ExpectedEqual(t, unsupportedMap["error"], "Action /unknown/action not supported on Obox app")

	// 4. Disconnect action
	actionDisconnect := QueueAction{
		UUID: "action-disc",
		Payload: ActionPayload{
			URL:    "/odoo/disconnect",
			Method: "GET",
		},
	}
	m.executeAction(actionDisconnect)
	testutil.ExpectedEqual(t, <-mockOdoo.ActionReported, "action-disc")
}

func TestObox_FetchNextActions(t *testing.T) {
	mockServer := testutil.NewMockOdooServer(t)
	mockServer.CustomHandler = func(w http.ResponseWriter, r *http.Request) bool {
		var req struct {
			Params struct {
				Token string `json:"token"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Params.Token == "valid-token" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"result": []QueueAction{
					{
						UUID: "action-uuid-101",
						Payload: ActionPayload{
							URL:    "/odoo/health",
							Method: "GET",
						},
					},
				},
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    404,
					"message": "Device not found",
				},
			})
		}
		return true
	}

	m, _ := createTestModule(t)

	// 1. Success case
	actions, err := m.fetchNextActions(mockServer.URL, "valid-token")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, actions, 1)
	testutil.ExpectedEqual(t, actions[0].UUID, "action-uuid-101")

	// 2. Error case (JSON-RPC 404)
	_, err = m.fetchNextActions(mockServer.URL, "invalid-token")
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, isDeviceNotFound(err))

	// 3. Raw HTTP 404 (non-RPC envelope)
	raw404Server := testutil.NewMockOdooServer(t)
	raw404Server.CustomHandler = func(w http.ResponseWriter, r *http.Request) bool {
		http.NotFound(w, r)
		return true
	}

	_, err = m.fetchNextActions(raw404Server.URL, "any-token")
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, isDeviceNotFound(err))
}
