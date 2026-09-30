package printer

import (
	"fmt"
	"sync"

	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"
)

type Manager struct {
	mu       sync.Mutex
	cfg      *config.Manager
	printers map[string]*Printer
}

type RawPrinter struct {
	ConnectionType ConnKind `json:"connectionType"`
	Address        string   `json:"address"`
	Name           string   `json:"name,omitempty"`
	Protocol       Protocol `json:"protocol"`
	BottomPadding  int      `json:"bottomPadding"`
}

func NewManager(cfg *config.Manager) *Manager {
	return &Manager{cfg: cfg, printers: make(map[string]*Printer)}
}

func (m *Manager) setProtocol(rawId string, printer *Printer) {
	resolveProtocol := getKnownPrinterByID
	switch getPrinterKind(rawId) {
	case ConnKindBT:
		resolveProtocol = m.getBluetoothPrinterConfig
	}

	printer.protocol, printer.bottomPadding = resolveProtocol(rawId)
}

func (m *Manager) Get(id string) (*Printer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.printers[id]; ok {
		logger.Debugf("Reusing existing printer instance for ID: %s", id)
		return p, nil
	}

	if address, ok := decodeBluetoothPrinterID(id); ok {
		if !m.isBluetoothPrinterConfigured(address) {
			return nil, fmt.Errorf("bluetooth printer %q is not configured", address)
		}
	}

	logger.Debugf("Creating new printer instance for ID: %s", id)
	p := newPrinter(id)
	if err := p.ensureOpen(); err != nil {
		return nil, fmt.Errorf("failed to open new printer instance for ID %s: %w", id, err)
	}
	m.setProtocol(id, p)

	m.printers[id] = p
	logger.Debugf("Registered new printer instance for ID: %s", id)
	return p, nil
}

func (m *Manager) WriteAsync(printerId string, data []byte) (<-chan JobResult, error) {
	p, err := m.Get(printerId)
	if err != nil {
		return nil, fmt.Errorf("failed to get printer for ID %s: %w", printerId, err)
	}

	return p.WriteAsync(data)
}
