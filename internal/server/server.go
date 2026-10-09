package server

import (
	"encoding/xml"
	"errors"
	"fmt"
	"sync/atomic"

	"obox-app/internal/escpos"
	"obox-app/internal/logger"
	"obox-app/internal/printer"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

type EPOSResponse struct {
	XMLName xml.Name `xml:"response"`
	Success bool     `xml:"success,attr"`
	Code    string   `xml:"code,attr"`
	Status  string   `xml:"status,attr"`
}

type Server struct {
	app     *fiber.App
	Port    int
	running atomic.Bool
}

func New(port int, mgr *printer.Manager) *Server {
	app := fiber.New(fiber.Config{AppName: "Obox App"})
	app.Use(cors.New(cors.Config{
		AllowOrigins:        []string{"*"},
		AllowPrivateNetwork: true,
	}))

	app.Get("/", func(ctx fiber.Ctx) error {
		return ctx.SendString(fmt.Sprintf("Hello from %s", app.Config().AppName))
	})

	app.Post("/p/:printerId/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Print request received for printer: %s", printerId)
		return printData(mgr, ctx, printerId)
	})

	app.Post("/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		logger.Debugf("Print request received (auto printer selection)")
		return printData(mgr, ctx, "")
	})

	app.Post("/p/:printerId/pstprnt", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Label print request received for printer: %s", printerId)
		return printLabel(mgr, ctx, printerId)
	})

	server := &Server{app: app, Port: port}
	server.running.Store(true)
	go func() {
		logger.Infof("HTTP server listening on 0.0.0.0:%d", port)
		err := app.Listen(fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			logger.Error("EPOS Server Error: ", err)
		}
		server.running.Store(false)
		logger.Warn("HTTP server stopped")
	}()
	return server
}

func printData(mgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	logger.Debugf("Processing print job for printer: %s", printerID)
	jobData, err := escpos.ParseXML(ctx.Body())
	if err != nil {
		logger.Errorf("XML parsing error: %v", err)
		return ctx.XML(EPOSResponse{Success: false, Code: "SchemaError", Status: ""})
	}
	logger.Debug("XML parsed successfully")

	reply, err := mgr.WriteAsync(printerID, jobData)
	if err == nil {
		logger.Debug("Print job queued")
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}
	if err != nil {
		retCode := ""
		if errors.Is(err, printer.ErrQueueFull) {
			retCode = "TooManyRequests"
			logger.Warn("Printer queue full")
		} else {
			retCode = "EX_BADPORT"
		}
		logger.Errorf("Print error [%s]: %v, Printer ID: %s", retCode, err, printerID)
		return ctx.XML(EPOSResponse{Success: false, Code: retCode, Status: ""})
	}
	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return ctx.XML(EPOSResponse{Success: true, Code: "", Status: ""})
}

func printLabel(mgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	jobData := ctx.Body()

	if len(jobData) == 0 {
		logger.Warn("Empty label data received")
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	logger.Debugf("Processing label print job for printer: %s", printerID)

	reply, err := mgr.WriteAsync(printerID, jobData)
	if err == nil {
		logger.Debug("Label print job queued")
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}

	if err != nil {
		if errors.Is(err, printer.ErrQueueFull) {
			logger.Warnf("Printer queue full, Printer ID: %s", printerID)
			return ctx.SendStatus(fiber.StatusTooManyRequests)
		}

		logger.Errorf("Print error: %v, Printer ID: %s", err, printerID)
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return ctx.SendStatus(fiber.StatusOK)
}

func (s *Server) Stop() error {
	logger.Infof("Stopping HTTP server")
	s.running.Store(false)
	return s.app.Shutdown()
}

func (s *Server) Running() bool {
	return s.running.Load()
}
