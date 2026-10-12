//go:build windows

package main

import (
	"context"
	"time"

	"obox-app/internal/logger"

	"golang.org/x/sys/windows/svc"
)

const defaultServiceName = "odoo-kiosk"

type eposWindowsService struct {
	app *App
}

func (s *eposWindowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	port, err := s.app.startBackend()
	if err != nil {
		logger.Errorf("Windows service failed to start backend: %v", err)
		return true, 1
	}

	logger.Infof("Windows service starting HTTP server on %s:%d", port)

	// Wait for server to become ready
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.app.webserver.WaitReady(ctx); err != nil {
		logger.Errorf("Windows service HTTP server startup wait failure: %v", err)
		return true, 2
	}

	logger.Infof("Windows service HTTP server ready on %d", port)
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			logger.Infof("Windows service received stop/shutdown request (%v)", c.Cmd)
			changes <- svc.Status{State: svc.StopPending}

			if s.app.webserver != nil {
				if err := s.app.webserver.Stop(); err != nil {
					logger.Errorf("Windows service webserver stop error: %v", err)
				}
			}

			logger.Infof("Windows service stopped cleanly")
			return false, 0
		default:
			logger.Warnf("Windows service unexpected control request #%d", c)
		}
	}
}

func isWindowsService() bool {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		logger.Warnf("Failed to check if process is Windows service: %v", err)
		return false
	}
	return isSvc
}

func runWindowsService(app *App) {
	logger.Infof("Starting application in native Windows Service mode")
	if err := svc.Run(defaultServiceName, &eposWindowsService{app: app}); err != nil {
		logger.Errorf("Windows service execution failed: %v", err)
	}
}
