package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"obox-app/internal/obox"
	"sync"
	"sync/atomic"

	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/printer"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// Server wraps the Fiber HTTP server with auth state.
type Server struct {
	app        *fiber.App
	Port       int
	running    atomic.Bool
	obox       *obox.Manager
	printerMgr *printer.Manager
	cfg        *config.Manager

	ready                    chan struct{}
	readyOnce                sync.Once
	reloadCount              atomic.Int64
	mu                       sync.RWMutex
	wailsToken               string          // trusted Wails session token (empty = none set yet)
	sessions                 map[string]bool // active PIN-auth session tokens (remote clients)
	onKioskChanged           func(enabled bool)
	onConfigChanged          func()
	onKioskReload            func()
	onKioskExit              func()
	onCompleteSessionPairing func(nonce, instanceID, token, dbURL string, oboxID int) error
	onOdooSessionExit        func()
	wailsAppURL              string
}

// SetOnCompleteSessionPairing sets a callback invoked when a session pairing is submitted via HTTP.
func (s *Server) SetOnCompleteSessionPairing(fn func(nonce, instanceID, token, dbURL string, oboxID int) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCompleteSessionPairing = fn
}

// SetOnOdooSessionExit sets a callback invoked when an Odoo session is exited from the webview session bar.
func (s *Server) SetOnOdooSessionExit(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onOdooSessionExit = fn
}

// New creates and starts the HTTP server on the specified bind host.
func New(port int, printerMgr *printer.Manager, obox *obox.Manager, cfg *config.Manager, distFS fs.FS) *Server {
	app := fiber.New(fiber.Config{AppName: "Obox app"})

	app.Use(func(c fiber.Ctx) error {
		// Chrome / Edge Local Network Access (LNA) and Private Network Access (PNA)
		c.Set("Access-Control-Allow-Private-Network", "true")
		c.Set("Access-Control-Allow-Local-Network", "true")
		c.Set("Permissions-Policy", "local-network=*, local-network-access=*, private-network-access=*, loopback-network=*")
		return c.Next()
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:        []string{"*"},
		AllowPrivateNetwork: true,
	}))

	server := &Server{
		app:        app,
		Port:       port,
		obox:       obox,
		printerMgr: printerMgr,
		ready:      make(chan struct{}),
		cfg:        cfg,
		sessions:   make(map[string]bool),
	}

	server.registerRoutes(distFS)
	server.running.Store(true)
	go func() {
		defer func() {
			server.running.Store(false)
			server.signalReady()
			logger.Warn("HTTP server stopped")
		}()

		logger.Infof("HTTP server listening on 0.0.0.0:%d", port)
		listenCfg := fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				server.signalReady()
			},
		}
		if err := app.Listen(fmt.Sprintf("0.0.0.0:%d", port), listenCfg); err != nil {
			logger.Error("EPOS Server Error: ", err)
		}
	}()
	return server
}

func (s *Server) Stop() error {
	logger.Infof("Stopping HTTP server")
	s.running.Store(false)
	return s.app.Shutdown()
}

func (s *Server) Running() bool {
	return s.running.Load()
}

func (s *Server) registerRoutes(distFS fs.FS) {
	s.app.Get("/", func(ctx fiber.Ctx) error {
		return ctx.SendString(fmt.Sprintf("Hello from %s", s.app.Config().AppName))
	})

	registerEPOSRoutes(s)
	registerOboxRoutes(s)
	registerAppRoutes(s)
	registerPrinterRoutes(s)
	registerWebViewRoutes(s)
	registerFrontendRoutes(s, distFS)
}

func (s *Server) signalReady() {
	s.readyOnce.Do(func() {
		close(s.ready)
	})
}

// WaitReady blocks until the HTTP server is listening or the context is cancelled.
func (s *Server) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		if !s.running.Load() {
			return errors.New("HTTP server failed to start")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// bindJSON is a tiny helper that unmarshals the request body into v.
func bindJSON(c fiber.Ctx, v any) error {
	return json.Unmarshal(c.Body(), v)
}
