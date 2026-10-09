package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrNoAvailablePort = errors.New("no available port in range")

const AppName = "OboxApp"

const (
	PortRangeStart = 4545
	PortRangeEnd   = 4555

	DebugModeDuration = 24 * time.Hour
)

type AppConfig struct {
	Port            int    `json:"port"`
	NetworkPrinting bool   `json:"network_printing"`
	LastSeenUpdate  string `json:"last_seen_update,omitempty"`

	LANPrinters []string `json:"lan_printers,omitempty"`

	DebugMode          bool       `json:"debug_mode"`
	DebugModeExpiresAt *time.Time `json:"debug_mode_expires_at,omitempty"`
}

func defaults() AppConfig {
	return AppConfig{
		Port:               0,
		NetworkPrinting:    false,
		DebugMode:          false,
		DebugModeExpiresAt: nil,
	}
}

type Manager struct {
	mu   sync.RWMutex
	path string
	Data AppConfig
}

func NewManager() (*Manager, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cannot locate user config dir: %w", err)
	}

	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create config dir: %w", err)
	}

	return &Manager{
		path: filepath.Join(dir, "config.json"),
		Data: defaults(),
	}, nil
}

func (cm *Manager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("config read error: %w", err)
	}

	if err := json.Unmarshal(data, &cm.Data); err != nil {
		return fmt.Errorf("config parse error: %w", err)
	}
	return nil
}

func (cm *Manager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.saveLocked()
}

func (cm *Manager) saveLocked() error {
	data, err := json.MarshalIndent(cm.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("config marshal error: %w", err)
	}
	if err := os.WriteFile(cm.path, data, 0644); err != nil {
		return fmt.Errorf("config write error: %w", err)
	}
	return nil
}

func (cm *Manager) Path() string { return cm.path }

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func findAvailablePort(start, end int) (int, error) {
	for p := start; p <= end; p++ {
		if isPortAvailable(p) {
			return p, nil
		}
	}

	return 0, fmt.Errorf("no available port found in range %d-%d: %w", start, end, ErrNoAvailablePort)
}

func (cm *Manager) ResolvePort() (int, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.Data.Port > 0 && isPortAvailable(cm.Data.Port) {
		return cm.Data.Port, nil
	}

	port, err := findAvailablePort(PortRangeStart, PortRangeEnd)
	if err != nil {
		return 0, err
	}

	cm.Data.Port = port
	if err := cm.saveLocked(); err != nil {
		log.Printf("[config] warning: could not save: %v\n", err)
	}
	return port, nil
}

func (cm *Manager) GetPort() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.Port
}

func (cm *Manager) SetNetworkPrintingEnabled(enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.NetworkPrinting = enabled
	return cm.saveLocked()
}

func (cm *Manager) IsNetworkPrintingEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.NetworkPrinting
}

func (cm *Manager) SetDebugMode(enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.DebugMode = enabled
	if enabled {
		exp := time.Now().Add(DebugModeDuration)
		cm.Data.DebugModeExpiresAt = &exp
	} else {
		cm.Data.DebugModeExpiresAt = nil
	}
	return cm.saveLocked()
}

func (cm *Manager) IsDebugMode() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.Data.DebugMode {
		if cm.Data.DebugModeExpiresAt == nil {
			exp := time.Now().Add(DebugModeDuration)
			cm.Data.DebugModeExpiresAt = &exp
			_ = cm.saveLocked()
			return true
		}
		if time.Now().After(*cm.Data.DebugModeExpiresAt) {
			cm.Data.DebugMode = false
			cm.Data.DebugModeExpiresAt = nil
			_ = cm.saveLocked()
			return false
		}
		return true
	}
	return false
}

func (cm *Manager) DebugModeExpiresAt() *time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if !cm.Data.DebugMode || cm.Data.DebugModeExpiresAt == nil {
		return nil
	}
	exp := *cm.Data.DebugModeExpiresAt
	return &exp
}

func (cm *Manager) AddLanEposPrinter(ip string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, existing := range cm.Data.LANPrinters {
		if existing == ip {
			return nil // Already exists
		}
	}
	cm.Data.LANPrinters = append(cm.Data.LANPrinters, ip)
	return cm.saveLocked()
}

func (cm *Manager) RemoveLANPrinter(ip string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, existing := range cm.Data.LANPrinters {
		if existing == ip {
			cm.Data.LANPrinters = append(cm.Data.LANPrinters[:i], cm.Data.LANPrinters[i+1:]...)
			return cm.saveLocked()
		}
	}
	return nil // Not found, nothing to remove
}

func (cm *Manager) GetLANPrinters() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.Data.LANPrinters == nil {
		return []string{}
	}
	// Return a copy to avoid races if caller modifies the slice
	result := make([]string, len(cm.Data.LANPrinters))
	copy(result, cm.Data.LANPrinters)
	return result
}

func (cm *Manager) LastSeenUpdate() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.LastSeenUpdate
}

func (cm *Manager) SetLastSeenUpdate(tag string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.Data.LastSeenUpdate = tag
	return cm.saveLocked()
}
