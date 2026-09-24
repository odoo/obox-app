package obox

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"obox-app/internal/config"
	"obox-app/internal/logger"
)

const lanContactTimeout = 30 * time.Second

type ConnectionState string

const (
	StateDisconnected ConnectionState = "disconnected"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
)

type ConnectionStatus struct {
	DBURL           string          `json:"dbUrl"`
	WebsocketStatus ConnectionState `json:"websocketStatus"`
	LanStatus       ConnectionState `json:"lanStatus"`
}

type ActionHandler func(ctx context.Context, action QueueAction) (any, error)

type Manager struct {
	appID string
	port  int
	cfg   *config.Manager

	credMu sync.RWMutex
	dbURL  string
	token  string
	handle ActionHandler

	wsStatus  atomic.Pointer[ConnectionState]
	lanStatus atomic.Pointer[ConnectionState]

	lastContactTime atomic.Int64
	lastLANContact  atomic.Int64

	lanMu    sync.Mutex
	lanTimer *time.Timer

	statusMu       sync.Mutex
	onStatusChange func(ConnectionStatus)

	connectMu     sync.Mutex
	connectCancel *cancelHandle

	workerMu     sync.Mutex
	workerCancel context.CancelFunc
	workerWg     sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(port int, cfg *config.Manager, handle ActionHandler) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		cfg:    cfg,
		port:   port,
		appID:  cfg.GetAppID(),
		handle: handle,
		ctx:    ctx,
		cancel: cancel,
	}

	m.setWsStatus(StateDisconnected)
	m.setLANStatus(StateDisconnected)

	if !cfg.HasOdooCredentials() {
		return m
	}

	odooCfg := cfg.GetOdooConfig()
	m.setCredentials(odooCfg.DBURL, odooCfg.Token, odooCfg.DbUUID)

	m.setWsStatus(StateConnecting)
	m.setLANStatus(StateConnecting)
	m.startQueueHandler()

	return m
}

func (m *Manager) startQueueHandler() {
	m.workerMu.Lock()
	defer m.workerMu.Unlock()

	if m.workerCancel != nil || m.ctx.Err() != nil {
		return
	}

	dbURL, token := m.getCredentials()
	if dbURL == "" || token == "" {
		return
	}

	ctx, cancel := context.WithCancel(m.ctx)
	m.workerCancel = cancel
	m.workerWg.Add(1)
	go func() {
		defer func() {
			m.workerMu.Lock()
			m.workerCancel = nil
			m.workerMu.Unlock()
			m.workerWg.Done()
		}()
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

type cancelHandle struct {
	cancel context.CancelFunc
}

func (m *Manager) cancelConnect() {
	m.connectMu.Lock()
	defer m.connectMu.Unlock()
	if m.connectCancel != nil {
		m.connectCancel.cancel()
		m.connectCancel = nil
	}
}

func (m *Manager) clearConnectCancel(h *cancelHandle) {
	if h == nil {
		return
	}
	h.cancel()
	m.connectMu.Lock()
	defer m.connectMu.Unlock()
	if m.connectCancel == h {
		m.connectCancel = nil
	}
}

func (m *Manager) setCredentials(dbURL, token, dbUUID string) {
	m.credMu.Lock()
	m.dbURL = dbURL
	m.token = token
	m.credMu.Unlock()

	if dbURL != "" && token != "" {
		if err := m.cfg.SaveOdooCredentials(dbURL, token, dbUUID); err != nil {
			logger.Warnf("[obox] Failed to save Odoo credentials to storage: %v", err)
		}
	}
}

func (m *Manager) getCredentials() (string, string) {
	m.credMu.RLock()
	defer m.credMu.RUnlock()

	return m.dbURL, m.token
}

func (m *Manager) clearCredentials() {
	m.setCredentials("", "", "")

	if err := m.cfg.ClearOdooConfig(); err != nil {
		logger.Warnf("[obox] Failed to clear Odoo credentials from storage: %v", err)
	}
}

func (m *Manager) getDbURL() string {
	m.credMu.RLock()
	defer m.credMu.RUnlock()

	return m.dbURL
}

func (m *Manager) getWebsocketStatus() ConnectionState {
	if status := m.wsStatus.Load(); status != nil {
		return *status
	}
	return StateDisconnected
}

func (m *Manager) getLANStatus() ConnectionState {
	if status := m.lanStatus.Load(); status != nil {
		if *status == StateConnected {
			last := m.lastLANContact.Load()
			if last == 0 || time.Since(time.UnixMilli(last)) >= lanContactTimeout {
				return StateDisconnected
			}
		}
		return *status
	}
	return StateDisconnected
}

func (m *Manager) LastLANContact() time.Time {
	last := m.lastLANContact.Load()
	if last == 0 {
		return time.Time{}
	}
	return time.UnixMilli(last)
}

func (m *Manager) recordLANContact() {
	m.lastLANContact.Store(time.Now().UnixMilli())
	m.setLANStatus(StateConnected)
}

func (m *Manager) cancelLANTimer() {
	m.lanMu.Lock()
	if m.lanTimer != nil {
		m.lanTimer.Stop()
		m.lanTimer = nil
	}

	m.lastLANContact.Store(0)
	changed := m.setLANStatusValueLocked(StateDisconnected)
	m.lanMu.Unlock()

	if changed {
		m.notifyStatusChange()
	}
}

func (m *Manager) clearConnection() {
	m.clearCredentials()
	m.setWsStatus(StateDisconnected)
	m.cancelLANTimer()
}

func (m *Manager) Disconnect() {
	logger.Infof("[obox] Disconnect triggered")

	m.cancelConnect()
	m.stopQueueHandler()
	m.clearConnection()
}

func (m *Manager) Stop() {
	m.cancelConnect()
	m.stopQueueHandler()
	if m.cancel != nil {
		m.cancel()
	}

	m.cancelLANTimer()
}

func (m *Manager) ConnectionStatus() ConnectionStatus {
	return ConnectionStatus{
		DBURL:           m.getDbURL(),
		WebsocketStatus: m.getWebsocketStatus(),
		LanStatus:       m.getLANStatus(),
	}
}

func (m *Manager) SetOnStatusChange(fn func(ConnectionStatus)) {
	m.statusMu.Lock()
	m.onStatusChange = fn
	m.statusMu.Unlock()
}

func (m *Manager) notifyStatusChange() {
	m.statusMu.Lock()
	fn := m.onStatusChange
	m.statusMu.Unlock()

	if fn != nil {
		fn(m.ConnectionStatus())
	}
}

func (m *Manager) setWsStatus(status ConnectionState) {
	prev := m.wsStatus.Load()
	m.wsStatus.Store(&status)

	if prev == nil || *prev != status {
		m.notifyStatusChange()
	}
}

func (m *Manager) setLANStatus(status ConnectionState) {
	m.lanMu.Lock()

	if m.lanTimer != nil {
		m.lanTimer.Stop()
		m.lanTimer = nil
	}

	var changed bool
	switch status {
	case StateConnected:
		if m.lastLANContact.Load() == 0 {
			m.lastLANContact.Store(time.Now().UnixMilli())
		}
		changed = m.setLANStatusValueLocked(StateConnected)
		m.lanTimer = time.AfterFunc(lanContactTimeout, func() {
			m.lanMu.Lock()
			last := m.lastLANContact.Load()
			var timerChanged bool
			if last == 0 || time.Since(time.UnixMilli(last)) >= lanContactTimeout {
				timerChanged = m.setLANStatusValueLocked(StateDisconnected)
			}
			m.lanMu.Unlock()

			if timerChanged {
				m.notifyStatusChange()
			}
		})
	case StateConnecting:
		changed = m.setLANStatusValueLocked(StateConnecting)
		m.lanTimer = time.AfterFunc(lanContactTimeout, func() {
			m.lanMu.Lock()
			var timerChanged bool
			if m.lanStatus.Load() != nil && *m.lanStatus.Load() == StateConnecting {
				timerChanged = m.setLANStatusValueLocked(StateDisconnected)
			}
			m.lanMu.Unlock()

			if timerChanged {
				m.notifyStatusChange()
			}
		})
	case StateDisconnected:
		m.lastLANContact.Store(0)
		changed = m.setLANStatusValueLocked(StateDisconnected)
	}
	m.lanMu.Unlock()

	if changed {
		m.notifyStatusChange()
	}
}

func (m *Manager) setLANStatusValueLocked(status ConnectionState) bool {
	prev := m.lanStatus.Load()
	m.lanStatus.Store(&status)
	return prev == nil || *prev != status
}
