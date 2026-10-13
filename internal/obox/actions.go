package obox

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"net/url"
	"os"
	"path/filepath"

	"github.com/kbinani/screenshot"

	"obox-app/internal/logger"
)

type openKioskPayload struct {
	URL      string `json:"url"`
	Timezone string `json:"timezone"`
}

var (
	screenshotFunc = defaultCaptureScreenshot
	readLogsFunc   = defaultReadAppLogs
)

func defaultCaptureScreenshot() (string, error) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return "", errors.New("no active display found for screenshot")
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return "", fmt.Errorf("failed to encode screenshot: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func defaultReadAppLogs() (string, error) {
	logDir := logger.LogDirectory()
	if logDir == "" {
		dir, err := os.UserConfigDir()
		if err == nil {
			logDir = filepath.Join(dir, "OboxApp", "logs")
		}
	}
	logPath := filepath.Join(logDir, "obox-app.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read log file: %w", err)
	}

	const maxLogBytes = 1024 * 1024
	if len(data) > maxLogBytes {
		data = data[len(data)-maxLogBytes:]
	}

	return base64.StdEncoding.EncodeToString(data), nil
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

	case "/odoo/sync":
		logger.Debugf("[obox queue] Action sync received")
		go func() {
			if err := m.Sync(m.ctx); err != nil {
				logger.Warnf("[obox queue] Action sync error: %v", err)
			}
		}()
		result = map[string]string{"status": "ok"}
		m.reportActionResult(action.UUID, result)
		return

	case "/odoo/kiosk_reload":
		logger.Debugf("[obox queue] Action kiosk_reload received")
		m.notifyReloadKiosk()
		result = map[string]string{"status": "ok"}
		m.reportActionResult(action.UUID, result)
		return

	case "/odoo/kiosk_screen_lock":
		logger.Debugf("[obox queue] Action kiosk_screen_lock received")
		payloadBytes := action.Payload.PayloadBytes()
		locked := false
		if len(payloadBytes) > 0 {
			var p struct {
				Locked bool `json:"locked"`
			}
			if err := json.Unmarshal(payloadBytes, &p); err == nil {
				locked = p.Locked
			}
		}
		logger.Infof("[obox queue] Action kiosk_screen_lock: locked=%v", locked)
		m.SetScreenLocked(locked)
		go func() {
			if err := m.Sync(m.ctx); err != nil {
				logger.Warnf("[obox queue] Sync error after screen lock change: %v", err)
			}
		}()
		result = map[string]string{"status": "ok"}
		m.reportActionResult(action.UUID, result)
		return

	case "/odoo/screenshot":
		logger.Debugf("[obox queue] Action screenshot received")
		screenshotBase64, err := screenshotFunc()
		if err != nil {
			logger.Errorf("[obox queue] Action screenshot error: %v", err)
			result = map[string]string{"error": err.Error()}
			m.reportActionResult(action.UUID, result)
			return
		}

		if err := m.sendScreenshot(screenshotBase64); err != nil {
			logger.Warnf("[obox queue] Failed to upload screenshot to Odoo: %v", err)
		}

		result = map[string]string{"status": "ok"}
		m.reportActionResult(action.UUID, result)
		return

	case "/odoo/logs":
		logger.Debugf("[obox queue] Action logs received")
		logsBase64, err := readLogsFunc()
		if err != nil {
			logger.Errorf("[obox queue] Action logs error: %v", err)
			result = map[string]string{"error": err.Error()}
			m.reportActionResult(action.UUID, result)
			return
		}

		if err := m.sendLogs(logsBase64); err != nil {
			logger.Warnf("[obox queue] Failed to upload logs to Odoo: %v", err)
		}

		result = map[string]string{"status": "ok"}
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
