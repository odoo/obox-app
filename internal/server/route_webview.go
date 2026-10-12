package server

import (
	"github.com/gofiber/fiber/v3"
)

type apiWebViewConfig struct {
	URL         string   `json:"url"`
	Enabled     bool     `json:"enabled"`
	HasPIN      bool     `json:"hasPIN"`
	ExitCorner  string   `json:"exitCorner"`
	ExitCorners []string `json:"exitCorners"`
	Fullscreen  bool     `json:"fullscreen"`
	ReloadCount int64    `json:"reloadCount"`
}

type setWebViewURLReq struct {
	URL string `json:"url"`
}

type setWebViewEnabledReq struct {
	Enabled bool `json:"enabled"`
}

type setWebViewExitCornerReq struct {
	Corner  string   `json:"corner"`
	Corners []string `json:"corners"`
}

type setWebViewFullscreenReq struct {
	Fullscreen bool `json:"fullscreen"`
}

func registerWebViewRoutes(s *Server) {
	s.app.Get("/api/webview", s.handleGetWebView)
	s.app.Post("/api/webview/url", s.requireAuth, s.handleSetWebViewURL)
	s.app.Post("/api/webview/enabled", s.requireAuth, s.handleSetWebViewEnabled)
	s.app.Post("/api/webview/corners", s.requireAuth, s.handleSetWebViewExitCorner)
	s.app.Post("/api/webview/fullscreen", s.requireAuth, s.handleSetWebViewFullscreen)
	s.app.Post("/api/webview/reload", s.requireAuth, s.handleReloadWebView)
	s.app.All("/api/kiosk/exit", s.handleKioskExit)
}

func (s *Server) handleGetWebView(c fiber.Ctx) error {
	if s.cfg == nil {
		return c.JSON(apiWebViewConfig{
			Fullscreen: true,
		})
	}
	return c.JSON(apiWebViewConfig{
		URL:         s.cfg.GetWebViewURL(),
		Enabled:     s.cfg.GetWebViewEnabled(),
		HasPIN:      s.cfg.HasWebViewPIN(),
		ExitCorner:  s.cfg.GetWebViewExitCorner(),
		ExitCorners: s.cfg.GetWebViewExitCorners(),
		Fullscreen:  s.cfg.GetWebViewFullscreen(),
		ReloadCount: s.reloadCount.Load(),
	})
}

func (s *Server) handleSetWebViewURL(c fiber.Ctx) error {
	var req setWebViewURLReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := s.cfg.SetWebViewURL(req.URL); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	s.mu.RLock()
	cb := s.onConfigChanged
	s.mu.RUnlock()
	if cb != nil {
		cb()
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleSetWebViewEnabled(c fiber.Ctx) error {
	var req setWebViewEnabledReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := s.cfg.SetWebViewEnabled(req.Enabled); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	s.mu.RLock()
	cb := s.onKioskChanged
	s.mu.RUnlock()
	if cb != nil {
		cb(req.Enabled)
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleSetWebViewExitCorner(c fiber.Ctx) error {
	var req setWebViewExitCornerReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	var err error
	if len(req.Corners) > 0 {
		err = s.cfg.SetWebViewExitCorners(req.Corners)
	} else if req.Corner != "" {
		err = s.cfg.SetWebViewExitCorner(req.Corner)
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "corner or corners is required"})
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	s.mu.RLock()
	cb := s.onConfigChanged
	s.mu.RUnlock()
	if cb != nil {
		cb()
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleSetWebViewFullscreen(c fiber.Ctx) error {
	var req setWebViewFullscreenReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := s.cfg.SetWebViewFullscreen(req.Fullscreen); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	s.mu.RLock()
	cb := s.onConfigChanged
	s.mu.RUnlock()
	if cb != nil {
		cb()
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleReloadWebView(c fiber.Ctx) error {
	s.reloadCount.Add(1)
	s.mu.RLock()
	cb := s.onKioskReload
	s.mu.RUnlock()
	if cb != nil {
		cb()
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleKioskExit(c fiber.Ctx) error {
	s.mu.RLock()
	cb := s.onKioskExit
	wailsURL := s.wailsAppURL
	s.mu.RUnlock()
	if cb != nil {
		go cb()
	}
	if c.Method() == fiber.MethodGet {
		if wailsURL == "" {
			wailsURL = "/"
		}
		return c.Redirect().Status(fiber.StatusTemporaryRedirect).To(wailsURL)
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// SetKioskCallback registers a callback invoked when kiosk enabled status changes via HTTP API.
func (s *Server) SetKioskCallback(cb func(bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onKioskChanged = cb
}

// SetConfigCallback registers a callback invoked when webview configuration changes via HTTP API.
func (s *Server) SetConfigCallback(cb func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onConfigChanged = cb
}

// SetKioskReloadCallback registers a callback invoked when kiosk reload is requested via HTTP API.
func (s *Server) SetKioskReloadCallback(cb func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onKioskReload = cb
}

// SetKioskExitCallback registers a callback invoked when exiting kiosk/webapp to return to the Wails app.
func (s *Server) SetKioskExitCallback(cb func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onKioskExit = cb
}

// SetWailsAppURL stores the local Wails UI URL used to redirect upon kiosk exit.
func (s *Server) SetWailsAppURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wailsAppURL = url
}
