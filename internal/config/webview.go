package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// GetWebViewURL returns the configured kiosk URL.
func (cm *Manager) GetWebViewURL() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewURL
}

// GetWebViewEnabled returns whether kiosk mode is enabled.
func (cm *Manager) GetWebViewEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewEnabled
}

// HasWebViewPIN reports whether a PIN has been configured.
func (cm *Manager) HasWebViewPIN() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewPIN != ""
}

// GetWebViewPIN returns the stored PIN (default "0000").
func (cm *Manager) GetWebViewPIN() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewPIN
}

// SetWebViewURL validates and persists the kiosk URL.
func (cm *Manager) SetWebViewURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed != "" {
		parsed, err := url.ParseRequestURI(trimmed)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("invalid URL: must be a valid HTTP or HTTPS address")
		}
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.WebViewURL = trimmed
	return cm.saveLocked()
}

// SetWebViewPIN validates (exactly 4 digits) and persists the plaintext PIN.
func (cm *Manager) SetWebViewPIN(pin string) error {
	if len(pin) != 4 {
		return errors.New("PIN must be exactly 4 digits")
	}
	for _, ch := range pin {
		if ch < '0' || ch > '9' {
			return errors.New("PIN must contain digits only")
		}
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.WebViewPIN = pin
	return cm.saveLocked()
}

// CheckWebViewPIN returns true when raw matches the stored PIN.
func (cm *Manager) CheckWebViewPIN(raw string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewPIN != "" && cm.Data.WebViewPIN == raw
}

// SetWebViewEnabled persists the enabled flag.
func (cm *Manager) SetWebViewEnabled(v bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if v && cm.Data.WebViewURL == "" {
		return errors.New("cannot enable kiosk mode: URL is not configured")
	}
	cm.Data.WebViewEnabled = v
	return cm.saveLocked()
}

// GetWebViewFullscreen returns whether kiosk mode runs in fullscreen. Defaults to true.
func (cm *Manager) GetWebViewFullscreen() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.Data.WebViewFullscreen
}

// SetWebViewFullscreen persists the fullscreen kiosk setting.
func (cm *Manager) SetWebViewFullscreen(fullscreen bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.WebViewFullscreen = fullscreen
	return cm.saveLocked()
}

var validCorners = map[string]bool{
	"top-left":     true,
	"top-right":    true,
	"bottom-left":  true,
	"bottom-right": true,
}

// GetWebViewExitCorners returns the configured exit gesture corners.
// Defaults to ["top-right"] if none are configured.
func (cm *Manager) GetWebViewExitCorners() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []string
	seen := make(map[string]bool)

	for _, c := range cm.Data.WebViewExitCorners {
		c = strings.ToLower(strings.TrimSpace(c))
		if validCorners[c] && !seen[c] {
			seen[c] = true
			result = append(result, c)
		}
	}

	if len(result) == 0 {
		return []string{"top-right"}
	}
	return result
}

// SetWebViewExitCorners validates and persists the list of active exit gesture corners.
func (cm *Manager) SetWebViewExitCorners(corners []string) error {
	var valid []string
	seen := make(map[string]bool)

	for _, c := range corners {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if !validCorners[c] {
			return fmt.Errorf("invalid exit corner: %q (must be top-right, top-left, bottom-right, or bottom-left)", c)
		}
		if !seen[c] {
			seen[c] = true
			valid = append(valid, c)
		}
	}

	if len(valid) == 0 {
		return errors.New("at least one exit corner must be selected")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.WebViewExitCorners = valid
	return cm.saveLocked()
}

// GetWebViewExitCorner returns a string representation of the configured exit corner(s).
func (cm *Manager) GetWebViewExitCorner() string {
	corners := cm.GetWebViewExitCorners()
	return strings.Join(corners, ",")
}

// SetWebViewExitCorner validates and persists exit gesture corner(s) (supports single or comma-separated).
func (cm *Manager) SetWebViewExitCorner(corner string) error {
	parts := strings.Split(corner, ",")
	return cm.SetWebViewExitCorners(parts)
}
