package obox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"epos-proxy/internal/logger"
)

type QueueAction struct {
	UUID    string        `json:"uuid"`
	Payload ActionPayload `json:"payload"`
}

type ActionPayload struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Payload json.RawMessage   `json:"payload"`
}

func (p ActionPayload) PayloadBytes() []byte {
	if len(p.Payload) == 0 || string(p.Payload) == "null" {
		return nil
	}
	var str string
	if json.Unmarshal(p.Payload, &str) == nil {
		return []byte(str)
	}
	return p.Payload
}

func (m *Manager) oboxQueueHandler(ctx context.Context) {
	logger.Debugf("[obox queue] Background polling worker started")
	defer logger.Debugf("[obox queue] Background polling worker stopped")

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		dbURL, token := m.GetCredentials()
		if dbURL == "" || token == "" {
			m.setWsStatus(StatusConnected)
			return
		}

		actions, err := m.fetchNextActions(dbURL, token)
		if err != nil {
			if isDeviceNotFound(err) {
				logger.Warnf("[obox queue] Device not found on server, clearing connection: %v", err)
				m.clearConnection()
				return
			}

			logger.Debugf("[obox queue] fetchNextActions: %v", err)

			last := m.lastContactTime.Load()
			if last == 0 || time.Since(time.UnixMilli(last)) > 15*time.Second {
				m.setWsStatus(StatusConnected)
			} else {
				m.setWsStatus(StatusConnecting)
			}

			timer.Reset(5 * time.Second)
			continue
		}

		m.setWsStatus(StatusConnected)
		m.lastContactTime.Store(time.Now().UnixMilli())

		for _, action := range actions {
			m.executeAction(action)
		}

		timer.Reset(5 * time.Second)
	}
}

func (m *Manager) fetchNextActions(dbURL, token string) ([]QueueAction, error) {
	resp, err := m.postJSONRPC(dbURL+"/obox/get_next_actions", map[string]string{"serial_number": m.appID, "token": token})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, &rpcError{Code: http.StatusNotFound, Message: "404 Not Found"}
		}
		return nil, fmt.Errorf("unexpected HTTP status %d from /obox/get_next_actions", resp.StatusCode)
	}

	var rpcResp struct {
		Result []QueueAction `json:"result"`
		Error  *rpcError     `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	return rpcResp.Result, nil
}

func (m *Manager) executeAction(action QueueAction) {
	actionPath := action.Payload.URL
	if parsed, err := url.Parse(actionPath); err == nil && parsed.Path != "" {
		actionPath = parsed.Path
	}

	logger.Infof("[obox queue] Executing queue action uuid=%s path=%s method=%s", action.UUID, actionPath, action.Payload.Method)

	var result interface{}

	switch {
	case actionPath == "/odoo/health":
		logger.Debugf("[obox queue] Action health ping: returning success")
		result = map[string]string{"status": "ok"}
		m.reportActionResult(action.UUID, result)
		go m.callOdooPing()
		return

	case actionPath == "/odoo/restart":
		logger.Warnf("[obox queue] Action restart requested: not supported on desktop Obox app")
		result = map[string]string{"error": "Restart is not supported on Obox app"}
		m.reportActionResult(action.UUID, result)
		return

	case actionPath == "/odoo/disconnect":
		logger.Debugf("[obox queue] Action disconnect: returning success")
		result = map[string]string{"status": "disconnected"}
		m.reportActionResult(action.UUID, result)
		m.Disconnect()
		return

	case actionPath == "/odoo/discover_devices":
		logger.Debugf("[obox queue] Action discover_devices: fetching device list")
		devices := m.buildDeviceList()
		m.reportActionResult(action.UUID, devices)
		return

	case strings.HasPrefix(actionPath, "/usb/v1/printer/") && strings.HasSuffix(actionPath, "/cgi-bin/epos/service.cgi"):
		result = m.printReceiptDirect(actionPath, action.Payload.PayloadBytes())
		m.reportActionResult(action.UUID, result)
		return

	default:
		logger.Warnf("[obox queue] Action %s: unsupported on desktop Obox app", actionPath)
		result = map[string]string{"error": fmt.Sprintf("Action %s not supported on Obox app", actionPath)}
		m.reportActionResult(action.UUID, result)
		return
	}
}
