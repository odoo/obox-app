package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockOdooServer wraps httptest.Server and provides channels for observing Odoo RPC interactions.
type MockOdooServer struct {
	*httptest.Server
	ActionReported chan string
	LastResult     chan string
	PingReceived   chan bool
	ConnectTokens  chan string
	PollCount      chan struct{}
	CustomHandler  func(w http.ResponseWriter, r *http.Request) bool
}

// NewMockOdooServer starts an httptest.Server that mocks standard Odoo obox JSON-RPC endpoints.
func NewMockOdooServer(t testing.TB) *MockOdooServer {
	t.Helper()
	mock := &MockOdooServer{
		ActionReported: make(chan string, 10),
		LastResult:     make(chan string, 10),
		PingReceived:   make(chan bool, 10),
		ConnectTokens:  make(chan string, 10),
		PollCount:      make(chan struct{}, 10),
	}

	mock.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if mock.CustomHandler != nil && mock.CustomHandler(w, r) {
			return
		}

		switch r.URL.Path {
		case "/obox/action_result":
			var req struct {
				Params struct {
					ActionUUID string `json:"action_uuid"`
					Result     string `json:"result"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Params.ActionUUID != "" {
				select {
				case mock.ActionReported <- req.Params.ActionUUID:
				default:
				}
				select {
				case mock.LastResult <- req.Params.Result:
				default:
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
		case "/obox/connect":
			var req struct {
				Params struct {
					Token string `json:"token"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			select {
			case mock.ConnectTokens <- req.Params.Token:
			default:
			}
			raw := json.RawMessage(`{"status":"paired"}`)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": &raw})
		case "/obox/ping":
			select {
			case mock.PingReceived <- true:
			default:
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
		case "/obox/get_next_actions":
			select {
			case mock.PollCount <- struct{}{}:
			default:
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": []interface{}{}})
		}
	}))
	t.Cleanup(mock.Close)
	return mock
}
