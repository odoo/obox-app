package obox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"epos-proxy/internal/logger"
	"epos-proxy/internal/util"
)

var httpClient = &http.Client{Timeout: 5 * time.Second}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"data"`
}

func (e *rpcError) Error() string {
	if e.Data.Name != "" {
		return fmt.Sprintf("server RPC error %d (%s): %s", e.Code, e.Data.Name, e.Message)
	}
	return fmt.Sprintf("server RPC error %d: %s", e.Code, e.Message)
}

func isDeviceNotFound(err error) bool {
	var rpcErr *rpcError
	if !errors.As(err, &rpcErr) || rpcErr == nil {
		return false
	}
	return rpcErr.Code == http.StatusNotFound || rpcErr.Data.Name == "werkzeug.exceptions.NotFound"
}

type rpcPayload struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      int         `json:"id"`
	Params  interface{} `json:"params"`
}

func (m *Manager) postJSONRPC(endpoint string, params interface{}) (*http.Response, error) {
	payload := rpcPayload{JSONRPC: "2.0", Method: "call", ID: 1, Params: params}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("json-rpc payload marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(m.ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return httpClient.Do(req)
}

func (m *Manager) reportActionResult(uuid string, result interface{}) {
	dbURL, token := m.GetCredentials()
	if dbURL == "" || token == "" {
		return
	}

	var resultStr string
	if b, err := json.Marshal(result); err == nil {
		resultStr = string(b)
	} else {
		resultStr = fmt.Sprintf("%v", result)
	}

	params := map[string]interface{}{
		"serial_number": m.appID,
		"token":         token,
		"action_uuid":   uuid,
		"result":        resultStr,
	}

	resp, err := m.postJSONRPC(dbURL+"/obox/action_result", params)
	if err != nil {
		logger.Errorf("[obox queue] action_result error for uuid %s: %v", uuid, err)
		return
	}
	defer resp.Body.Close()
	logger.Debugf("[obox queue] action_result response status: %d for uuid %s", resp.StatusCode, uuid)
}

func (m *Manager) callOdooPing() {
	dbURL, token := m.GetCredentials()
	if dbURL == "" || token == "" {
		return
	}

	resp, err := m.postJSONRPC(dbURL+"/obox/ping", map[string]string{"serial_number": m.appID, "token": token})
	if err != nil {
		logger.Errorf("[obox queue] /obox/ping error: %v", err)
		return
	}
	defer resp.Body.Close()
	logger.Debugf("[obox queue] /obox/ping response status: %d", resp.StatusCode)
}

func (m *Manager) callOdooOboxConnect(dbURL, token string) {
	endpoint := dbURL + "/obox/connect"

	for attempt := 1; attempt <= 10; attempt++ {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(time.Duration(attempt*300) * time.Millisecond):
		}

		params := map[string]interface{}{
			"serial_number": m.appID,
			"token":         token,
			"local_ip":      util.LocalAddr(m.port, m.cfg.IsNetworkPrintingEnabled()),
			"services":      []string{"usb", "printer"},
		}

		resp, err := m.postJSONRPC(endpoint, params)
		if err != nil {
			logger.Warnf("[obox] /obox/connect attempt %d connection error: %v", attempt, err)
			continue
		}

		var rpcResp struct {
			Result *json.RawMessage `json:"result"`
			Error  *json.RawMessage `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&rpcResp)
		resp.Body.Close()

		if resp.StatusCode == 200 && rpcResp.Error == nil {
			logger.Debugf("[obox] Odoo /obox/connect SUCCESS on attempt %d (paired as %s)", attempt, m.appID)
			m.setWsStatus("connected")
			return
		}

		if rpcResp.Error != nil {
			logger.Warnf("[obox] /obox/connect (serial=%s) error from Odoo: %s", m.appID, string(*rpcResp.Error))
		} else {
			logger.Warnf("[obox] /obox/connect (serial=%s) HTTP %d", m.appID, resp.StatusCode)
		}
	}

	logger.Errorf("[obox] Failed to complete /obox/connect after 10 attempts")
}
