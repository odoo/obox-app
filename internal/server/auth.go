package server

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// SetSessionToken registers the trusted Wails session token.
// Called once from App.startup() after the token is generated.
func (s *Server) SetSessionToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wailsToken = token
}

// CreatePINSession validates pin against the stored PIN. On success it issues
// an opaque session token the remote client can use for privileged calls.
// Returns ("", false) when pin is wrong or no PIN is configured.
func (s *Server) CreatePINSession(pin string) (string, bool) {
	if s.cfg == nil || !s.cfg.CheckWebViewPIN(pin) {
		return "", false
	}
	token := uuid.New().String()
	s.mu.Lock()
	s.sessions[token] = true
	s.mu.Unlock()
	return token, true
}

// isAuthenticated returns true when c carries either the Wails token or a
// valid PIN-session token.
func (s *Server) isAuthenticated(c fiber.Ctx) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Trusted Wails request
	if wt := c.Get("X-Wails-Token"); wt != "" && s.wailsToken != "" && wt == s.wailsToken {
		return true
	}

	// Remote PIN-session
	auth := c.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return s.sessions[strings.TrimPrefix(auth, "Bearer ")]
	}
	return false
}

// requireAuth is a Fiber middleware that blocks unauthenticated remote requests.
func (s *Server) requireAuth(c fiber.Ctx) error {
	if s.isAuthenticated(c) {
		return c.Next()
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
}
