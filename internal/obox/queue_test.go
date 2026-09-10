package obox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"epos-proxy/internal/testutil"
)

func TestObox_executeAction(t *testing.T) {
	actionReported := make(chan string, 10)
	lastResult := make(chan interface{}, 10)

	mockOdoo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				ActionUUID string      `json:"action_uuid"`
				Result     interface{} `json:"result"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.ActionUUID != "" {
			actionReported <- req.Params.ActionUUID
			lastResult <- req.Params.Result
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	}))
	defer mockOdoo.Close()

	m, _ := createTestModule(t)
	m.SetCredentials(mockOdoo.URL, "tok", "uuid")

	// 1. Health ping action
	actionHealth := QueueAction{
		UUID: "action-health",
		Payload: ActionPayload{
			URL:    "/odoo/health",
			Method: "GET",
		},
	}
	m.executeAction(actionHealth)
	testutil.ExpectedEqual(t, <-actionReported, "action-health")
	resHealth := <-lastResult
	var healthMap map[string]interface{}
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resHealth.(string)), &healthMap))
	testutil.ExpectedEqual(t, healthMap["status"], "ok")

	// 2. Discover devices action
	actionDiscover := QueueAction{
		UUID: "action-discover",
		Payload: ActionPayload{
			URL:    "/odoo/discover_devices",
			Method: "GET",
		},
	}
	m.executeAction(actionDiscover)
	testutil.ExpectedEqual(t, <-actionReported, "action-discover")
	resDiscover := <-lastResult
	var devList []map[string]interface{}
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resDiscover.(string)), &devList))

	// 3. Restart action (not supported)
	actionRestart := QueueAction{
		UUID: "action-restart",
		Payload: ActionPayload{
			URL:    "/odoo/restart",
			Method: "GET",
		},
	}
	m.executeAction(actionRestart)
	testutil.ExpectedEqual(t, <-actionReported, "action-restart")
	resRestart := <-lastResult
	var restartMap map[string]interface{}
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resRestart.(string)), &restartMap))
	testutil.ExpectedEqual(t, restartMap["error"], "Restart is not supported on Obox app")

	// 4. Remote debug action (not supported)
	actionDebug := QueueAction{
		UUID: "action-debug",
		Payload: ActionPayload{
			URL:    "/sos/v1/enable?token=123",
			Method: "GET",
		},
	}
	m.executeAction(actionDebug)
	testutil.ExpectedEqual(t, <-actionReported, "action-debug")
	resDebug := <-lastResult
	var debugMap map[string]interface{}
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resDebug.(string)), &debugMap))
	testutil.ExpectedNotEqual(t, debugMap["error"], "")

	// 5. Direct ePOS print action (mimic direct printing)
	actionPrint := QueueAction{
		UUID: "action-print-1",
		Payload: ActionPayload{
			URL:     "/usb/v1/printer/ipp_bDoxMjcuMC4wLjE/cgi-bin/epos/service.cgi",
			Method:  "POST",
			Payload: json.RawMessage(`"<epos-print><text>Receipt</text></epos-print>"`),
		},
	}
	m.executeAction(actionPrint)
	testutil.ExpectedEqual(t, <-actionReported, "action-print-1")
	resPrint := <-lastResult
	var printResStr string
	testutil.ExpectedNoError(t, json.Unmarshal([]byte(resPrint.(string)), &printResStr))
	testutil.ExpectedContains(t, printResStr, `response`)

	// 6. Disconnect action
	actionDisconnect := QueueAction{
		UUID: "action-disc",
		Payload: ActionPayload{
			URL:    "/odoo/disconnect",
			Method: "GET",
		},
	}
	m.executeAction(actionDisconnect)
	testutil.ExpectedEqual(t, <-actionReported, "action-disc")
}

func TestObox_FetchNextActions(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				SerialNumber string `json:"serial_number"`
				Token        string `json:"token"`
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
	}))
	defer mockServer.Close()

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
	raw404Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer raw404Server.Close()

	_, err = m.fetchNextActions(raw404Server.URL, "any-token")
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, isDeviceNotFound(err))
}
