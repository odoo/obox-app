package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OdooInstance represents a configured Odoo server/database instance.
type OdooInstance struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Name      string `json:"name,omitempty"`
	Token     string `json:"token,omitempty"`
	OboxID    int    `json:"obox_id,omitempty"`
	IsActive  bool   `json:"is_active"`
	CreatedAt int64  `json:"created_at"`
}

// isLocalOrPrivateHost checks if the given host is localhost, loopback, or private IP.
func isLocalOrPrivateHost(hostname string) bool {
	if hostname == "localhost" || strings.HasSuffix(hostname, ".local") {
		return true
	}
	ip := net.ParseIP(hostname)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// NormalizeOdooURL validates and normalizes an Odoo instance URL.
// HTTPS is enforced by default; HTTP is allowed only for localhost and private/dev networks.
func NormalizeOdooURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", errors.New("URL cannot be empty")
	}

	// Auto-prepend scheme if missing
	if !strings.Contains(trimmed, "://") {
		if strings.HasPrefix(trimmed, "localhost") || strings.HasPrefix(trimmed, "127.0.0.1") {
			trimmed = "http://" + trimmed
		} else {
			trimmed = "https://" + trimmed
		}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", errors.New("invalid URL format: must include a valid host")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("unsupported protocol: only HTTP and HTTPS are permitted")
	}

	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()

	// Enforce HTTPS security requirement: HTTP allowed only on local/private networks
	if scheme == "http" && !isLocalOrPrivateHost(host) {
		return "", errors.New("insecure HTTP is only permitted for localhost or local networks; please use HTTPS")
	}

	hostPart := host
	if port != "" {
		hostPart = net.JoinHostPort(host, port)
	}

	// Clean path: strip trailing slash and common web endpoints
	cleanPath := strings.TrimRight(parsed.Path, "/")
	cleanPath = strings.TrimSuffix(cleanPath, "/web")
	cleanPath = strings.TrimSuffix(cleanPath, "/odoo")

	normalized := fmt.Sprintf("%s://%s%s", scheme, hostPart, cleanPath)
	return strings.TrimRight(normalized, "/"), nil
}

// GetOdooInstances returns a list of all configured Odoo instances.
func (cm *Manager) GetOdooInstances() []OdooInstance {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.Data.OdooInstances) == 0 {
		return []OdooInstance{}
	}
	res := make([]OdooInstance, len(cm.Data.OdooInstances))
	copy(res, cm.Data.OdooInstances)
	return res
}

// GetActiveOdooInstance returns the currently active Odoo instance, or nil if none.
func (cm *Manager) GetActiveOdooInstance() *OdooInstance {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, inst := range cm.Data.OdooInstances {
		if inst.IsActive {
			instCopy := inst
			return &instCopy
		}
	}
	return nil
}

// GetOdooInstance returns the Odoo instance with the given ID, or nil if not found.
func (cm *Manager) GetOdooInstance(id string) *OdooInstance {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, inst := range cm.Data.OdooInstances {
		if inst.ID == id {
			instCopy := inst
			return &instCopy
		}
	}
	return nil
}

// AddOdooInstance normalizes the URL and adds a new Odoo instance entry.
// Prevents duplicate entries with the same normalized URL.
func (cm *Manager) AddOdooInstance(rawURL, name string) (OdooInstance, error) {
	normURL, err := NormalizeOdooURL(rawURL)
	if err != nil {
		return OdooInstance{}, err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, existing := range cm.Data.OdooInstances {
		if existing.URL == normURL {
			return OdooInstance{}, fmt.Errorf("an instance with URL %s is already configured", normURL)
		}
	}

	displayName := strings.TrimSpace(name)
	if displayName == "" {
		if parsed, err := url.Parse(normURL); err == nil {
			displayName = parsed.Host
		} else {
			displayName = normURL
		}
	}

	newInstance := OdooInstance{
		ID:        uuid.New().String(),
		URL:       normURL,
		Name:      displayName,
		CreatedAt: time.Now().Unix(),
	}

	cm.Data.OdooInstances = append(cm.Data.OdooInstances, newInstance)
	if err := cm.saveLocked(); err != nil {
		return OdooInstance{}, err
	}

	return newInstance, nil
}

// RemoveOdooInstance removes an instance by ID and clears active credentials if it was active.
func (cm *Manager) RemoveOdooInstance(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	idx := -1
	var removed OdooInstance
	for i, inst := range cm.Data.OdooInstances {
		if inst.ID == id {
			idx = i
			removed = inst
			break
		}
	}

	if idx == -1 {
		return errors.New("odoo instance not found")
	}

	cm.Data.OdooInstances = append(cm.Data.OdooInstances[:idx], cm.Data.OdooInstances[idx+1:]...)

	if removed.IsActive || cm.Data.Odoo.DBURL == removed.URL {
		cm.Data.Odoo = OdooConfig{}
	}

	return cm.saveLocked()
}

// SetActiveOdooInstance switches the active Odoo instance.
func (cm *Manager) SetActiveOdooInstance(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var target *OdooInstance
	for i := range cm.Data.OdooInstances {
		if cm.Data.OdooInstances[i].ID == id {
			cm.Data.OdooInstances[i].IsActive = true
			target = &cm.Data.OdooInstances[i]
		} else {
			cm.Data.OdooInstances[i].IsActive = false
		}
	}

	if target == nil {
		return errors.New("odoo instance not found")
	}

	if target.Token != "" {
		cm.Data.Odoo = OdooConfig{
			DBURL: target.URL,
			Token: target.Token,
		}
	} else {
		cm.Data.Odoo = OdooConfig{}
	}

	return cm.saveLocked()
}

// SaveOdooInstancePairing saves pairing results for the given instance and marks it active.
func (cm *Manager) SaveOdooInstancePairing(id string, token string, oboxID int) error {
	if token == "" {
		return errors.New("pairing token cannot be empty")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	var target *OdooInstance
	for i := range cm.Data.OdooInstances {
		if cm.Data.OdooInstances[i].ID == id {
			cm.Data.OdooInstances[i].Token = token
			cm.Data.OdooInstances[i].OboxID = oboxID
			cm.Data.OdooInstances[i].IsActive = true
			target = &cm.Data.OdooInstances[i]
		} else {
			cm.Data.OdooInstances[i].IsActive = false
		}
	}

	if target == nil {
		return errors.New("odoo instance not found")
	}

	cm.Data.Odoo = OdooConfig{
		DBURL: target.URL,
		Token: token,
	}

	return cm.saveLocked()
}
