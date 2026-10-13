package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// MockOdooServer wraps httptest.Server and provides channels for observing Odoo RPC interactions.
type MockOdooServer struct {
	*httptest.Server
	ActionReported        chan string
	LastResult            chan string
	PingReceived          chan bool
	ConnectTokens         chan string
	ConnectParamsReceived chan map[string]interface{}
	PollCount             chan struct{}
	ScreenshotReceived    chan string
	LogsReceived          chan string
	SyncReceived          chan map[string]interface{}
	FeatureRequested      chan string

	mu             sync.RWMutex
	configResponse map[string]interface{}
	CustomHandler  func(w http.ResponseWriter, r *http.Request) bool
}

// SetConfigResponse overrides the JSON-RPC result returned by /obox/sync and /obox/connect.
func (m *MockOdooServer) SetConfigResponse(resp map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configResponse = resp
}

func (m *MockOdooServer) getConfigResponse() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.configResponse != nil {
		return m.configResponse
	}
	return map[string]interface{}{
		"config_up_to_date": true,
		"config_version":    "mock-v1",
	}
}

// NewMockOdooServer starts an httptest.Server that mocks standard Odoo obox JSON-RPC endpoints.
func NewMockOdooServer(t testing.TB) *MockOdooServer {
	t.Helper()
	mock := &MockOdooServer{
		ActionReported:        make(chan string, 10),
		LastResult:            make(chan string, 10),
		PingReceived:          make(chan bool, 10),
		ConnectTokens:         make(chan string, 10),
		ConnectParamsReceived: make(chan map[string]interface{}, 10),
		PollCount:             make(chan struct{}, 10),
		ScreenshotReceived:    make(chan string, 10),
		LogsReceived:          make(chan string, 10),
		SyncReceived:          make(chan map[string]interface{}, 10),
		FeatureRequested:      make(chan string, 10),
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

		case "/obox/screenshot":
			var req struct {
				Params struct {
					Screenshot string `json:"screenshot"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			select {
			case mock.ScreenshotReceived <- req.Params.Screenshot:
			default:
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})

		case "/obox/logs":
			var req struct {
				Params struct {
					Logs string `json:"logs"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			select {
			case mock.LogsReceived <- req.Params.Logs:
			default:
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})

		case "/obox/connect":
			var req struct {
				Params map[string]interface{} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if tok, ok := req.Params["token"].(string); ok {
				select {
				case mock.ConnectTokens <- tok:
				default:
				}
			}
			select {
			case mock.ConnectParamsReceived <- req.Params:
			default:
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": mock.getConfigResponse()})

		case "/obox/sync":
			var req struct {
				Params struct {
					SerialNumber string                 `json:"serial_number"`
					Token        string                 `json:"token"`
					State        map[string]interface{} `json:"state"`
					Full         bool                   `json:"full"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Params.State != nil {
				select {
				case mock.SyncReceived <- req.Params.State:
				default:
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": mock.getConfigResponse()})

		case "/obox/feature":
			var req struct {
				Params struct {
					Name string `json:"name"`
				} `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			select {
			case mock.FeatureRequested <- req.Params.Name:
			default:
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]interface{}{}})

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
