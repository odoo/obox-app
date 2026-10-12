package server

import (
	"fmt"

	"obox-app/internal/logger"
	"obox-app/internal/printer"
	"obox-app/internal/util"

	"github.com/gofiber/fiber/v3"
)

type apiPrinter struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Id     string `json:"id"`
	IsLAN  bool   `json:"isLAN"`
	LANIp  string `json:"lanIp,omitempty"`
	Online bool   `json:"online"`
	Type   string `json:"type"`
}

type apiUnavailablePrinter struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type apiPrintersResponse struct {
	ErrorMsg            string                  `json:"errorMsg"`
	Printers            []apiPrinter            `json:"printers"`
	UnavailablePrinters []apiUnavailablePrinter `json:"unavailablePrinters"`
}

type addLANPrinterReq struct {
	IP string `json:"ip"`
}

type removeLANPrinterReq struct {
	IP string `json:"ip"`
}

func registerPrinterRoutes(s *Server) {
	s.app.Get("/api/printers", s.handleGetPrinters)
	s.app.Get("/api/printers/lan/:ip/status", s.handleGetLANPrinterStatus)
	s.app.Post("/api/printers/lan", s.requireAuth, s.handleAddLANPrinter)
	s.app.Delete("/api/printers/lan", s.requireAuth, s.handleRemoveLANPrinter)
}

func (s *Server) handleGetPrinters(c fiber.Ctx) error {
	printers := make([]apiPrinter, 0)
	unavailable := make([]apiUnavailablePrinter, 0)
	errorMsg := ""

	printerURL := func(id string) string {
		if s.cfg == nil {
			return fmt.Sprintf("localhost:%d/p/%s", s.Port, id)
		}
		return fmt.Sprintf("%s:%d/p/%s", util.GetLocalIP(s.cfg.IsNetworkPrintingEnabled()), s.Port, id)
	}

	infos, err := printer.ListUSBPrinters()
	if err == nil {
		logger.Debugf("API: detected %d available USB printers", len(infos.Available))
		for _, info := range infos.Available {
			printers = append(printers, apiPrinter{
				Id:     info.Id,
				Name:   info.Name,
				Ip:     printerURL(info.Id),
				Online: true,
				Type:   string(info.Type),
			})
		}
		for _, info := range infos.Unavailable {
			unavailable = append(unavailable, apiUnavailablePrinter{
				Name:     info.Name,
				ErrorMsg: info.Error,
			})
		}
	} else {
		errorMsg = err.Error()
		logger.Errorf("API: USB printer detection failed: %v", err)
	}

	if s.cfg != nil {
		for _, info := range printer.ListLANPrinters(s.cfg) {
			printers = append(printers, apiPrinter{
				Id:    info.Id,
				Name:  fmt.Sprintf("Network - %s", info.IP),
				Ip:    printerURL(info.Id),
				IsLAN: true,
				LANIp: info.IP,
				Type:  string(printer.TypeReceipt),
			})
		}
	}

	return c.JSON(apiPrintersResponse{
		Printers:            printers,
		UnavailablePrinters: unavailable,
		ErrorMsg:            errorMsg,
	})
}

func (s *Server) handleGetLANPrinterStatus(c fiber.Ctx) error {
	ip := c.Params("ip")
	online := printer.CheckLANPrinter(ip) == nil
	return c.JSON(fiber.Map{"online": online})
}

func (s *Server) handleAddLANPrinter(c fiber.Ctx) error {
	var req addLANPrinterReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	ip, err := printer.ValidateIPAddress(req.IP)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid IP address: " + err.Error()})
	}

	if err := printer.CheckLANPrinter(ip); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "LAN printer unreachable: " + err.Error()})
	}

	if err := s.cfg.AddLanEposPrinter(ip); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save: " + err.Error()})
	}

	logger.Debugf("API: LAN printer added: %s", ip)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (s *Server) handleRemoveLANPrinter(c fiber.Ctx) error {
	var req removeLANPrinterReq
	if err := bindJSON(c, &req); err != nil || req.IP == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if err := s.cfg.RemoveLANPrinter(req.IP); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	logger.Debugf("API: LAN printer removed: %s", req.IP)
	return c.JSON(fiber.Map{"ok": true})
}
