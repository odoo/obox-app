package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"obox-app/internal/logger"
)

func runServerMode(app *App) {
	logger.Infof("Starting application in server/kiosk mode")

	// Ensure auto-start on Windows startup is enabled in server mode
	if runtime.GOOS == "windows" {
		if !app.AppVariable().AutoStart {
			if err := app.ToggleAutoStart(); err != nil {
				logger.Warnf("Failed to configure auto-start on Windows startup: %v", err)
			} else {
				logger.Infof("Configured auto-start on Windows startup for server mode")
			}
		}
	}

	port, err := app.startBackend()
	if err != nil {
		logger.Errorf("HTTP server startup failure: %v", err)
		return
	}

	logger.Infof("Starting HTTP server on 0.0.0.0:%d", port)

	// Wait for server to become ready
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.webserver.WaitReady(ctx); err != nil {
		logger.Errorf("HTTP server startup failure: %v", err)
		return
	}

	logger.Infof("HTTP server ready")

	// Keep process alive until termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Infof("Shutting down server mode")
	if app.webserver != nil {
		if err := app.webserver.Stop(); err != nil {
			logger.Errorf("Server stop error: %v", err)
		}
	}
}
