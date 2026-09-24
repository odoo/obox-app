package obox

import (
	"context"
	"obox-app/internal/logger"

	"github.com/gofiber/fiber/v3"
)

func (m *Manager) HandleLanConnection(ctx fiber.Ctx) error {
	logger.Debug("[obox] LAN health check /odoo/")

	if dbURL, _ := m.getCredentials(); dbURL != "" {
		m.recordLANContact()
		return ctx.Status(fiber.StatusOK).JSON(map[string]interface{}{
			"status": "configured",
			"data": map[string]string{
				"serial": m.appID,
				"db_url": dbURL,
			},
		})
	}

	m.setLANStatus(StateDisconnected)
	return ctx.Status(fiber.StatusServiceUnavailable).JSON(map[string]interface{}{"status": "not_configured"})
}

func (m *Manager) HandleOfflineConnect(ctx fiber.Ctx) error {
	dbURL := ctx.Query("db_url")
	token := ctx.Query("token")
	dbUUID := ctx.Query("db_uuid")

	logger.Debugf("[obox] offline connect received: db_url=%s, db_uuid=%s", dbURL, dbUUID)
	if dbURL == "" || token == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(map[string]interface{}{
			"error": "missing required parameters: db_url and token",
		})
	}

	m.connect(dbURL, token, dbUUID)
	return ctx.SendStatus(fiber.StatusOK)
}

func (m *Manager) connect(dbURL, token, dbUUID string) {
	m.cancelConnect()
	m.stopQueueHandler()
	m.setCredentials(dbURL, token, dbUUID)

	ctx, cancel := context.WithCancel(m.ctx)
	handle := &cancelHandle{cancel: cancel}

	m.connectMu.Lock()
	m.connectCancel = handle
	m.connectMu.Unlock()

	m.setWsStatus(StateConnecting)

	go func() {
		defer m.clearConnectCancel(handle)

		if err := m.callOdooOboxConnect(ctx, dbURL, token); err != nil {
			if ctx.Err() == nil {
				logger.Errorf("[obox] Connection failed: %v", err)
				m.setWsStatus(StateDisconnected)
			}
			return
		}

		m.startQueueHandler()
	}()
}

func (m *Manager) HandleDisconnect(ctx fiber.Ctx) error {
	logger.Debugf("[obox] /odoo/disconnect request received")
	m.Disconnect()
	return ctx.JSON(map[string]interface{}{"status": "disconnected"})
}
