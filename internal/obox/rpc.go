package obox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"obox-app/internal/logger"
	"obox-app/internal/util"
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

func (m *Manager) postJSONRPCWithContext(ctx context.Context, endpoint string, params interface{}) (*http.Response, error) {
	payload := rpcPayload{JSONRPC: "2.0", Method: "call", ID: 1, Params: params}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("json-rpc payload marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return httpClient.Do(req)
}

func (m *Manager) postJSONRPC(endpoint string, params interface{}) (*http.Response, error) {
	return m.postJSONRPCWithContext(m.ctx, endpoint, params)
}

func (m *Manager) reportActionResult(uuid string, result interface{}) {
	dbURL, token := m.getCredentials()
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

func (m *Manager) sendScreenshot(screenshotBase64 string) error {
	dbURL, token := m.getCredentials()
	if dbURL == "" || token == "" {
		return errors.New("no Odoo credentials")
	}

	params := map[string]interface{}{
		"serial_number": m.appID,
		"token":         token,
		"screenshot":    screenshotBase64,
	}

	resp, err := m.postJSONRPC(dbURL+"/obox/screenshot", params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (m *Manager) sendLogs(logsBase64 string) error {
	dbURL, token := m.getCredentials()
	if dbURL == "" || token == "" {
		return errors.New("no Odoo credentials")
	}

	params := map[string]interface{}{
		"serial_number": m.appID,
		"token":         token,
		"logs":          logsBase64,
	}

	resp, err := m.postJSONRPC(dbURL+"/obox/logs", params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (m *Manager) callOdooPing() {
	dbURL, token := m.getCredentials()
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

func (m *Manager) callOdooOboxConnect(ctx context.Context, dbURL, token string) error {
	endpoint := dbURL + "/obox/connect"

	for attempt := 1; attempt <= 10; attempt++ {
		if attempt > 1 {
			delay := time.Duration(attempt-1) * time.Second
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		params := map[string]interface{}{
			"serial_number": m.appID,
			"token":         token,
			"local_ip":      util.LocalAddr(m.port, m.cfg.IsNetworkPrintingEnabled()),
			"services":      []string{"usb", "printer", "kiosk", "sync"},
			"local_port":    m.port,
			"name":          "OBOX App",
			"platform":      "android",
		}

		resp, err := m.postJSONRPCWithContext(ctx, endpoint, params)
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
			m.setWsStatus(StateConnected)

			if rpcResp.Result != nil {
				var configResp ConfigResponse
				if err := json.Unmarshal(*rpcResp.Result, &configResp); err == nil {
					m.applyConfigResponse(&configResp)
				}
			}

			go m.syncDeviceState(ctx, dbURL, token)
			return nil
		}

		if rpcResp.Error != nil {
			logger.Warnf("[obox] /obox/connect (serial=%s) error from Odoo: %s", m.appID, string(*rpcResp.Error))
		} else {
			logger.Warnf("[obox] /obox/connect (serial=%s) HTTP %d", m.appID, resp.StatusCode)
		}
	}

	logger.Errorf("[obox] Failed to complete /obox/connect after 10 attempts")
	return fmt.Errorf("failed to complete /obox/connect after 10 attempts")
}

// ConfigResponse represents the Odoo _config_response payload.
type ConfigResponse struct {
	ConfigUpToDate bool               `json:"config_up_to_date"`
	ConfigVersion  string             `json:"config_version"`
	Config         *OdooConfigPayload `json:"config"`
}

// OdooConfigPayload represents configuration fields pushed by Odoo.
type OdooConfigPayload struct {
	Name     string        `json:"name"`
	KioskPIN string        `json:"kiosk_pin"`
	Features []interface{} `json:"features"`
	Devices  []interface{} `json:"devices"`
}

func (m *Manager) applyConfigResponse(resp *ConfigResponse) {
	if resp == nil {
		return
	}
	if resp.ConfigVersion != "" {
		m.SetAppliedConfigVersion(resp.ConfigVersion)
	}
	if resp.Config != nil && resp.Config.KioskPIN != "" {
		if resp.Config.KioskPIN != m.cfg.GetWebViewPIN() {
			logger.Infof("[obox] Applying kiosk PIN from Odoo config: %s", resp.Config.KioskPIN)
			_ = m.cfg.SetWebViewPIN(resp.Config.KioskPIN)
		}
	}
}

// Sync transmits current device state to Odoo's /obox/sync endpoint.
func (m *Manager) Sync(ctx context.Context) error {
	dbURL, token := m.getCredentials()
	if dbURL == "" || token == "" {
		return errors.New("no Odoo credentials")
	}
	return m.syncDeviceState(ctx, dbURL, token)
}

func (m *Manager) syncDeviceState(ctx context.Context, dbURL, token string) error {
	state := m.BuildDeviceState()
	params := map[string]interface{}{
		"serial_number": m.appID,
		"token":         token,
		"state":         state,
	}
	endpoint := dbURL + "/obox/sync"
	resp, err := m.postJSONRPCWithContext(ctx, endpoint, params)
	if err != nil {
		logger.Debugf("[obox] /obox/sync error: %v", err)
		return err
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result *json.RawMessage `json:"result"`
		Error  *json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err == nil && rpcResp.Result != nil {
		var configResp ConfigResponse
		if err := json.Unmarshal(*rpcResp.Result, &configResp); err == nil {
			m.applyConfigResponse(&configResp)
		}
	}

	if rpcResp.Error != nil {
		logger.Warnf("[obox] /obox/sync error from Odoo: %s", string(*rpcResp.Error))
		return fmt.Errorf("odoo error: %s", string(*rpcResp.Error))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from /obox/sync", resp.StatusCode)
	}

	logger.Infof("[obox] /obox/sync successfully completed for serial=%s", m.appID)
	return nil
}
