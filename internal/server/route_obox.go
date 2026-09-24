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
}
