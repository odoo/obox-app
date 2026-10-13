package server

import (
	"obox-app/internal/logger"

	"github.com/gofiber/fiber/v3"
)

func registerOboxRoutes(s *Server) {
	s.app.Get("/odoo/", s.obox.HandleLanConnection)
	s.app.Get("/odoo/connect", s.obox.HandleOfflineConnect)
	s.app.Get("/odoo/disconnect", s.obox.HandleDisconnect)

	// Direct call using the printer IP generated from the OBOX record.
	s.app.Post("/usb/v1/printer/:printerId/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Print request received for printer: %s", printerId)
		return printReceipt(s.printerMgr, ctx, printerId)
	})

	s.app.Post("/api/odoo/complete_session_pairing", func(ctx fiber.Ctx) error {
		var req struct {
			Nonce      string `json:"nonce"`
			InstanceID string `json:"instanceID"`
			Token      string `json:"token"`
			DbURL      string `json:"dbURL"`
			OboxID     int    `json:"oboxID"`
		}
		if err := bindJSON(ctx, &req); err != nil {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		s.mu.RLock()
		fn := s.onCompleteSessionPairing
		s.mu.RUnlock()
		if fn != nil {
			if err := fn(req.Nonce, req.InstanceID, req.Token, req.DbURL, req.OboxID); err != nil {
				return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	s.app.All("/api/odoo/exit_session", func(ctx fiber.Ctx) error {
		s.mu.RLock()
		fn := s.onOdooSessionExit
		s.mu.RUnlock()
		if fn != nil {
			fn()
		}
		return ctx.JSON(fiber.Map{"status": "ok"})
	})
}
