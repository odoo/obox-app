package server

import (
	"os"
	"runtime"
	"time"

	"obox-app/internal/logger"
	"obox-app/internal/util"

	"github.com/gofiber/fiber/v3"
)

type apiAppVariable struct {
	ServerRunning bool   `json:"serverRunning"`
	Os            string `json:"os"`
	Fullscreen    bool   `json:"fullscreen"`
}

type apiTroubleshootInfo struct {
	ActiveFirewall string `json:"activeFirewall"`
	FirewallZone   string `json:"firewallZone"`
	Port           int    `json:"port"`
	Subnet         string `json:"subnet"`
	LocalIP        string `json:"localIp"`
	ExecPath       string `json:"execPath"`
}

type authSessionReq struct {
	PIN string `json:"pin"`
}

func registerAppRoutes(s *Server) {
	s.app.Get("/api/app", s.handleGetApp)
	s.app.Get("/api/troubleshoot", s.handleGetTroubleshoot)
	s.app.Post("/api/auth/session", s.handleAuthSession)
	s.app.Post("/api/app/quit", s.requireAuth, s.handleQuitApp)
}

func (s *Server) handleGetApp(c fiber.Ctx) error {
	fullscreen := false
	if s.cfg != nil {
		fullscreen = s.cfg.GetWebViewFullscreen()
	}
	return c.JSON(apiAppVariable{
		ServerRunning: true,
		Os:            runtime.GOOS,
		Fullscreen:    fullscreen,
	})
}

func (s *Server) handleGetTroubleshoot(c fiber.Ctx) error {
	netInfo := util.GetNetworkInfo()
	execPath, _ := os.Executable()
	port := 0
	if s.cfg != nil {
		port = s.cfg.GetPort()
	}
	return c.JSON(apiTroubleshootInfo{
		ActiveFirewall: netInfo.ActiveFirewall,
		FirewallZone:   netInfo.Zone,
		Port:           port,
		Subnet:         netInfo.Subnet,
		LocalIP:        netInfo.IP,
		ExecPath:       execPath,
	})
}

func (s *Server) handleAuthSession(c fiber.Ctx) error {
	if s.cfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "service unavailable"})
	}

	var req authSessionReq
	if err := bindJSON(c, &req); err != nil || req.PIN == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	token, ok := s.CreatePINSession(req.PIN)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid PIN"})
	}

	return c.JSON(fiber.Map{"token": token})
}

func (s *Server) handleQuitApp(c fiber.Ctx) error {
	logger.Infof("Quit application requested via API")
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = s.Stop()
		os.Exit(0)
	}()
	return c.JSON(fiber.Map{"ok": true})
}
