package obox

import (
	"epos-proxy/internal/logger"

	"github.com/gofiber/fiber/v3"
)

func (m *Manager) HandleLanConnection(ctx fiber.Ctx) error {
	logger.Debug("[obox] LAN health check /odoo/")

	if dbURL, _ := m.GetCredentials(); dbURL != "" {
		m.RecordLANContact()
		return ctx.Status(fiber.StatusOK).JSON(map[string]interface{}{
			"status": "configured",
			"data": map[string]string{
				"serial": m.appID,
				"db_url": dbURL,
			},
		})
	}

	m.setLANStatus(StatusDisconnected)
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

	m.SetCredentials(dbURL, token, dbUUID)
	m.setWsStatus(StatusConnecting)
	go m.callOdooOboxConnect(dbURL, token)

	return ctx.SendStatus(fiber.StatusOK)
}

func (m *Manager) HandleDisconnect(ctx fiber.Ctx) error {
	logger.Debugf("[obox] /odoo/disconnect request received")
	m.Disconnect()
	return ctx.JSON(map[string]interface{}{"status": "disconnected"})
}
