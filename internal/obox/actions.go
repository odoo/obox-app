package obox

import (
	"fmt"
	"net/url"

	"epos-proxy/internal/logger"
)

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
