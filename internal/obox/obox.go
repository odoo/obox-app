package obox

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"
)

const lanContactTimeout = 30 * time.Second

type StatusType string

const (
	StatusDisconnected StatusType = "disconnected"
	StatusConnecting   StatusType = "connecting"
	StatusConnected    StatusType = "connected"
)

type ConnectionStatus struct {
	DBURL           string     `json:"dbUrl"`
	WebsocketStatus StatusType `json:"websocketStatus"`
	LanStatus       StatusType `json:"lanStatus"`
}

type Manager struct {
	appID   string
	port    int
	cfg     *config.Manager
	printer *printer.Manager

	credMu sync.RWMutex
	dbURL  string
	token  string

	wsStatus  atomic.Pointer[StatusType]
	lanStatus atomic.Pointer[StatusType]

	lastContactTime atomic.Int64
	lastLANContact  atomic.Int64

	lanMu    sync.Mutex
	lanTimer *time.Timer

	statusMu       sync.RWMutex
	onStatusChange func(ConnectionStatus)

	workerMu     sync.Mutex
	workerCancel context.CancelFunc
	workerWg     sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(port int, cfg *config.Manager, printer *printer.Manager) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		cfg:     cfg,
		port:    port,
		appID:   cfg.GetAppID(),
		printer: printer,
		ctx:     ctx,
		cancel:  cancel,
	}

	m.setWsStatus(StatusDisconnected)
	m.setLANStatus(StatusDisconnected)

	if !cfg.HasOdooCredentials() {
		return m
	}

	odooCfg := cfg.GetOdooConfig()
	m.setCredentials(odooCfg.DBURL, odooCfg.Token)

	m.setWsStatus(StatusConnecting)
	m.setLANStatus(StatusConnecting)
	m.startQueueHandler()

	return m
}

func (m *Manager) startQueueHandler() {
	m.workerMu.Lock()
	defer m.workerMu.Unlock()

	if m.workerCancel != nil || m.ctx.Err() != nil {
		return
	}

	dbURL, token := m.GetCredentials()
	if dbURL == "" || token == "" {
		return
	}

	ctx, cancel := context.WithCancel(m.ctx)
	m.workerCancel = cancel
	m.workerWg.Add(1)
	go func() {
		defer m.workerWg.Done()
		m.oboxQueueHandler(ctx)
	}()
}

func (m *Manager) stopQueueHandler() {
	m.workerMu.Lock()
	cancel := m.workerCancel
	m.workerCancel = nil
	m.workerMu.Unlock()

	if cancel != nil {
		cancel()
		m.workerWg.Wait()
	}
}

func (m *Manager) SetCredentials(dbURL, token, dbUUID string) {
	m.setCredentials(dbURL, token)
	if err := m.cfg.SetOdooCredentials(dbURL, token, dbUUID); err != nil {
		logger.Warnf("[obox] Failed to save Odoo credentials to storage: %v", err)
	}

	m.startQueueHandler()
}

func (m *Manager) setCredentials(dbURL, token string) {
	m.credMu.Lock()
	m.dbURL = dbURL
	m.token = token
	m.credMu.Unlock()
}

func (m *Manager) GetCredentials() (string, string) {
	m.credMu.RLock()
	defer m.credMu.RUnlock()

	return m.dbURL, m.token
}

func (m *Manager) ClearCredentials() {
	m.setCredentials("", "")

	if err := m.cfg.ClearOdooConfig(); err != nil {
		logger.Warnf("[obox] Failed to clear Odoo credentials from storage: %v", err)
	}
}

func (m *Manager) GetDbURL() string {
	m.credMu.RLock()
	defer m.credMu.RUnlock()

	return m.dbURL
}

func (m *Manager) GetWebsocketStatus() StatusType {
	if status := m.wsStatus.Load(); status != nil {
		return *status
	}
	return StatusDisconnected
}

func (m *Manager) GetLANStatus() StatusType {
	if status := m.lanStatus.Load(); status != nil {
		if *status == StatusConnected {
			last := m.lastLANContact.Load()
			if last == 0 || time.Since(time.UnixMilli(last)) >= lanContactTimeout {
				return StatusDisconnected
			}
		}
		return *status
	}
	return StatusDisconnected
}

func (m *Manager) LastLANContact() time.Time {
	last := m.lastLANContact.Load()
	if last == 0 {
		return time.Time{}
	}
	return time.UnixMilli(last)
}

func (m *Manager) RecordLANContact() {
	m.lastLANContact.Store(time.Now().UnixMilli())
	m.setLANStatus(StatusConnected)
}

func (m *Manager) cancelLANTimer() {
	m.lanMu.Lock()
	defer m.lanMu.Unlock()

	if m.lanTimer != nil {
		m.lanTimer.Stop()
		m.lanTimer = nil
	}

	m.lastLANContact.Store(0)
	m.setLANStatusValue(StatusDisconnected)
}

func (m *Manager) clearConnection() {
	m.ClearCredentials()
	m.setWsStatus(StatusDisconnected)
	m.cancelLANTimer()
}

func (m *Manager) Disconnect() {
	logger.Infof("[obox] Disconnect triggered")

	m.stopQueueHandler()
	m.clearConnection()
}

func (m *Manager) Stop() {
	m.stopQueueHandler()
	if m.cancel != nil {
		m.cancel()
	}

	m.cancelLANTimer()
}

func (m *Manager) ConnectionStatus() ConnectionStatus {
	return ConnectionStatus{
		DBURL:           m.GetDbURL(),
		WebsocketStatus: m.GetWebsocketStatus(),
		LanStatus:       m.GetLANStatus(),
	}
}

func (m *Manager) SetOnStatusChange(fn func(ConnectionStatus)) {
	m.statusMu.Lock()
	m.onStatusChange = fn
	m.statusMu.Unlock()
}

func (m *Manager) notifyStatusChange() {
	m.statusMu.RLock()
	fn := m.onStatusChange
	m.statusMu.RUnlock()

	if fn != nil {
		status := m.ConnectionStatus()
		go fn(status)
	}
}

func (m *Manager) setWsStatus(status StatusType) {
	prev := m.wsStatus.Load()
	m.wsStatus.Store(&status)

	if prev == nil || *prev != status {
		m.notifyStatusChange()
	}
}

func (m *Manager) setLANStatus(status StatusType) {
	m.lanMu.Lock()
	defer m.lanMu.Unlock()

	if m.lanTimer != nil {
		m.lanTimer.Stop()
		m.lanTimer = nil
	}

	switch status {
	case StatusConnected:
		if m.lastLANContact.Load() == 0 {
			m.lastLANContact.Store(time.Now().UnixMilli())
		}
		m.setLANStatusValue(StatusConnected)
		m.lanTimer = time.AfterFunc(lanContactTimeout, func() {
			m.lanMu.Lock()
			defer m.lanMu.Unlock()
			last := m.lastLANContact.Load()
			if last == 0 || time.Since(time.UnixMilli(last)) >= lanContactTimeout {
				m.setLANStatusValue(StatusDisconnected)
			}
		})
	case StatusConnecting:
		m.setLANStatusValue(StatusConnecting)
		m.lanTimer = time.AfterFunc(lanContactTimeout, func() {
			m.lanMu.Lock()
			defer m.lanMu.Unlock()
			if m.lanStatus.Load() != nil && *m.lanStatus.Load() == StatusConnecting {
				m.setLANStatusValue(StatusDisconnected)
			}
		})
	case StatusDisconnected:
		m.lastLANContact.Store(0)
		m.setLANStatusValue(StatusDisconnected)
	}
}

func (m *Manager) setLANStatusValue(status StatusType) {
	prev := m.lanStatus.Load()
	m.lanStatus.Store(&status)

	if prev == nil || *prev != status {
		m.notifyStatusChange()
	}
}
