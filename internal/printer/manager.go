package printer

import (
	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/util"
	"fmt"
	"sync"
)

type Manager struct {
	mu       sync.Mutex
	port     int
	cfg      *config.Manager
	printers map[string]*Printer
}

func NewManager(port int, cfg *config.Manager) *Manager {
	return &Manager{port: port, cfg: cfg, printers: make(map[string]*Printer)}
}

func (m *Manager) Get(id string) (*Printer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.printers[id]; ok {
		logger.Debugf("Reusing existing printer instance for ID: %s", id)
		return p, nil
	}

	logger.Debugf("Creating new printer instance for ID: %s", id)
	p := newPrinter(id)
	if err := p.ensureOpen(); err != nil {
		return nil, fmt.Errorf("failed to open new printer instance for ID %s: %w", id, err)
	}

	m.printers[id] = p
	logger.Debugf("Registered new printer instance for ID: %s", id)
	return p, nil
}

func (m *Manager) WriteAsync(printerId string, data []byte) (<-chan JobResult, error) {
	p, err := m.Get(printerId)
	if err != nil {
		return nil, fmt.Errorf("failed to get printer for ID %s: %w", printerId, err)
	}

	reply := make(chan JobResult, 1)
	err = p.Enqueue(func(p *Printer) JobResult {
		logger.Debugf("Executing print job for printer %s", printerId)
		if err := p.Write(data); err != nil {
			return JobResult{Err: fmt.Errorf("print job failed for printer %s: %w", printerId, err)}
		}
		logger.Debugf("Print job completed for printer %s", printerId)
		return JobResult{OK: true}
	}, reply)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue print job for printer %s: %w", printerId, err)
	}

	return reply, nil
}

type Device struct {
	Ip         string `json:"ip"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsLAN      bool   `json:"isLAN"`
	LANIp      string `json:"lanIp,omitempty"`
	Online     bool   `json:"online"`
}

type UnavailableDevice struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type DiscoveryResult struct {
	Printers            []Device            `json:"printers"`
	UnavailablePrinters []UnavailableDevice `json:"unavailablePrinters"`
	ErrorMsg            string              `json:"errorMsg"`
}

func (m *Manager) DiscoverAllPrinters() DiscoveryResult {
	available := make([]Device, 0)
	unavailable := make([]UnavailableDevice, 0)
	var scanErr string

	// 1. USB printers
	usbPrinters, err := ListUSBPrinters()
	if err != nil {
		scanErr = err.Error()
		logger.Errorf("USB printer detection failed: %v", err)
	} else if usbPrinters != nil {
		logger.Debugf("Detected %d available USB printers", len(usbPrinters.Available))
		for _, info := range usbPrinters.Available {
			available = append(available, Device{
				Ip:         util.GetPrinterUrl(m.port, m.cfg.IsNetworkPrintingEnabled(), info.Id),
				Identifier: info.Id,
				Name:       info.Name,
				Type:       string(info.Type),
				IsLAN:      false,
				Online:     true,
			})
		}

		for _, u := range usbPrinters.Unavailable {
			unavailable = append(unavailable, UnavailableDevice{
				Name:     u.Name,
				ErrorMsg: u.Error,
			})
			logger.Warnf("USB printer unavailable: %s (%s)", u.Name, u.Error)
		}
	}

	// 2. LAN printers
	lanPrinters := ListLANPrinters(m.cfg)
	for _, lan := range lanPrinters {
		available = append(available, Device{
			Ip:         util.GetPrinterUrl(m.port, m.cfg.IsNetworkPrintingEnabled(), lan.Id),
			Identifier: lan.Id,
			Name:       fmt.Sprintf("Network - %s", lan.IP),
			Type:       string(TypeReceipt),
			IsLAN:      true,
			LANIp:      lan.IP,
			Online:     true,
		})
	}

	return DiscoveryResult{
		Printers:            available,
		UnavailablePrinters: unavailable,
		ErrorMsg:            scanErr,
	}
}
