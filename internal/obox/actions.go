package obox

import (
	"encoding/json"
	"fmt"
	"net/url"

	"obox-app/internal/logger"
)

type openKioskPayload struct {
	URL      string `json:"url"`
	Timezone string `json:"timezone"`
}

func (m *Manager) executeAction(action QueueAction) {
	actionPath := action.Payload.URL
	if parsed, err := url.Parse(actionPath); err == nil && parsed.Path != "" {
		actionPath = parsed.Path
		action.Payload.URL = actionPath
	}

	logger.Debugf("[obox queue] Executing queue action uuid=%s path=%s method=%s", action.UUID, actionPath, action.Payload.Method)

	var result interface{}

	switch actionPath {
	case "/odoo/health":
		logger.Debugf("[obox queue] Action health ping: returning success")
		result = map[string]string{"status": "ok"}
		m.reportActionResult(action.UUID, result)
		go m.callOdooPing()
		return

	case "/odoo/restart":
		logger.Warnf("[obox queue] Action restart requested: not supported on desktop Obox app")
		result = map[string]string{"error": "Restart is not supported on Obox app"}
		m.reportActionResult(action.UUID, result)
		return

	case "/odoo/disconnect":
		logger.Debugf("[obox queue] Action disconnect: returning success")
		result = map[string]string{"status": "disconnected"}
		m.reportActionResult(action.UUID, result)
		m.Disconnect()
		return

	case "/odoo/open_kiosk":
		logger.Debugf("[obox queue] Action open_kiosk received")
		payloadBytes := action.Payload.PayloadBytes()
		var kioskURL string
		var tz string
		if len(payloadBytes) > 0 {
			var data openKioskPayload
			if err := json.Unmarshal(payloadBytes, &data); err == nil && data.URL != "" {
				kioskURL = data.URL
				tz = data.Timezone
			} else {
				var rawURL string
				if err := json.Unmarshal(payloadBytes, &rawURL); err == nil && rawURL != "" {
					kioskURL = rawURL
				}
			}
		}

		if kioskURL == "" {
			logger.Warnf("[obox queue] Action open_kiosk: missing or empty URL in payload %s", string(payloadBytes))
			result = map[string]string{"error": "missing url in payload"}
			m.reportActionResult(action.UUID, result)
			return
		}

		logger.Infof("[obox queue] Opening kiosk URL: %s (timezone: %s)", kioskURL, tz)
		if m.cfg != nil {
			if err := m.cfg.SetWebViewURL(kioskURL); err != nil {
				logger.Errorf("[obox queue] Action open_kiosk failed to set URL: %v", err)
				result = map[string]string{"error": err.Error()}
				m.reportActionResult(action.UUID, result)
				return
			}
			if err := m.cfg.SetWebViewEnabled(true); err != nil {
				logger.Errorf("[obox queue] Action open_kiosk failed to enable kiosk: %v", err)
				result = map[string]string{"error": err.Error()}
				m.reportActionResult(action.UUID, result)
				return
			}
		}

		m.notifyOpenKiosk(kioskURL)

		result = map[string]string{"status": "opened"}
		m.reportActionResult(action.UUID, result)
		return

	default:
		if m.handle != nil {
			handlerResult, err := m.handle(m.ctx, action)
			if err != nil {
				logger.Errorf("[obox queue] Action %s error: %v", actionPath, err)
				m.reportActionResult(action.UUID, map[string]string{"error": err.Error()})
				return
			}
			if handlerResult != nil {
				m.reportActionResult(action.UUID, handlerResult)
				return
			}
		}
		logger.Warnf("[obox queue] Action %s: unsupported on desktop Obox app", actionPath)
		result = map[string]string{"error": fmt.Sprintf("Action %s not supported on Obox app", actionPath)}
		m.reportActionResult(action.UUID, result)
		return
	}
}
