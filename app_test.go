package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"obox-app/internal/app_actions"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/obox"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/testutil"
	"obox-app/internal/util"
)

type recordedQuestion struct {
	Title        string
	Message      string
	ConfirmLabel string
	CancelLabel  string
}

type recordedSave struct {
	Title             string
	Filename          string
	FilterDisplayName string
	FilterPattern     string
}

type recordedError struct {
	Title   string
	Message string
}

// fakeDialogs is a dialoger that returns canned responses and records every
// invocation, so dialog-driven code paths can be tested without Wails.
type fakeDialogs struct {
	errors       []recordedError
	questionResp bool
	questions    []recordedQuestion
	savePath     string
	saveErr      error
	saves        []recordedSave
}

func (f *fakeDialogs) Error(title, message string) {
	f.errors = append(f.errors, recordedError{Title: title, Message: message})
}

func (f *fakeDialogs) Question(title, message, confirmLabel, cancelLabel string) bool {
	f.questions = append(f.questions, recordedQuestion{
		Title:        title,
		Message:      message,
		ConfirmLabel: confirmLabel,
		CancelLabel:  cancelLabel,
	})
	return f.questionResp
}

func (f *fakeDialogs) SaveFile(title, filename, filterDisplayName, filterPattern string) (string, error) {
	f.saves = append(f.saves, recordedSave{
		Title:             title,
		Filename:          filename,
		FilterDisplayName: filterDisplayName,
		FilterPattern:     filterPattern,
	})
	return f.savePath, f.saveErr
}

type emittedEvent struct {
	Name string
	Data []any
}

// fakeEvents is an emitter that records every emitted event, so event-driven
// code paths can be tested without Wails.
type fakeEvents struct {
	mu      sync.Mutex
	emitted []emittedEvent
}

func (f *fakeEvents) Emit(name string, data ...any) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emitted = append(f.emitted, emittedEvent{
		Name: name,
		Data: data,
	})
	return true
}

func (f *fakeEvents) getEvents() []emittedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]emittedEvent, len(f.emitted))
	copy(copied, f.emitted)
	return copied
}

func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func TestNewApp(t *testing.T) {
	app := NewApp()
	testutil.ExpectedNotNil(t, app)
	testutil.ExpectedNotNil(t, app.events)
	testutil.ExpectedNotNil(t, app.config)
}

func TestApp_CheckOdooStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	cfg.Data.Port = testutil.GetFreePort(t)
	_ = cfg.SaveOdooCredentials("http://127.0.0.1:8069", "tok")

	mgr := printer.NewManager(cfg)
	oboxMod := obox.NewManager(cfg.Data.Port, cfg, app_actions.OboxActionHandler(mgr))
	srv := server.New(cfg.Data.Port, mgr, oboxMod)
	defer srv.Stop()
	defer oboxMod.Stop()

	app := &App{
		webserver:      srv,
		config:         cfg,
		printerManager: mgr,
		oboxMgr:        oboxMod,
	}

	status := app.CheckOdooStatus()
	testutil.ExpectedEqual(t, status.DBURL, "http://127.0.0.1:8069")
	testutil.ExpectedEqual(t, status.LanStatus, "connecting")
}

func TestApp_ConfirmDisconnectOdoo(t *testing.T) {
	tests := []struct {
		name             string
		questionResponse bool
		expectDisconnect bool
	}{
		{name: "confirm disconnect", questionResponse: true, expectDisconnect: true},
		{name: "cancel disconnect", questionResponse: false, expectDisconnect: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			cfg, err := config.NewManager()
			testutil.ExpectedNoError(t, err)
			cfg.Data.Port = testutil.GetFreePort(t)
			_ = cfg.SaveOdooCredentials("http://127.0.0.1:8069", "tok")

			mgr := printer.NewManager(cfg)
			oboxMod := obox.NewManager(cfg.Data.Port, cfg, app_actions.OboxActionHandler(mgr))
			srv := server.New(cfg.Data.Port, mgr, oboxMod)
			defer srv.Stop()
			defer oboxMod.Stop()

			dialogs := &fakeDialogs{questionResp: tc.questionResponse}
			events := &fakeEvents{}
			app := &App{
				webserver:      srv,
				config:         cfg,
				printerManager: mgr,
				oboxMgr:        oboxMod,
				dialogs:        dialogs,
				events:         events,
			}
			oboxMod.SetOnStatusChange(func(status obox.ConnectionStatus) {
				app.events.Emit("odoo:status_changed", status)
			})

			disconnected, err := app.ConfirmDisconnectOdoo()
			testutil.ExpectedNoError(t, err)
			testutil.ExpectedEqual(t, disconnected, tc.expectDisconnect)

			if tc.expectDisconnect {
				testutil.ExpectedFalse(t, cfg.HasOdooCredentials())
				var evts []emittedEvent
				passed := waitFor(500*time.Millisecond, func() bool {
					evts = events.getEvents()
					return len(evts) > 0
				})
				testutil.ExpectedTrue(t, passed, "expected status_changed event to be emitted")
				testutil.ExpectedEqual(t, evts[0].Name, "odoo:status_changed")
			} else {
				testutil.ExpectedTrue(t, cfg.HasOdooCredentials())
				testutil.ExpectedLen(t, events.getEvents(), 0)
			}
		})
	}
}

func TestApp_ConfirmDisconnectOdoo_NilObox(t *testing.T) {
	app := &App{dialogs: &fakeDialogs{}}
	disconnected, err := app.ConfirmDisconnectOdoo()
	testutil.ExpectedError(t, err)
	testutil.ExpectedFalse(t, disconnected)
	testutil.ExpectedEqual(t, err.Error(), "Odoo connection is not initialized")
}

func TestApp_AppVariableAndPrintersAndGetPrinterUrl(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	err = cfg.AddLanEposPrinter("192.168.1.100")
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager(cfg)
	srv := server.New(port, mgr, nil)
	defer srv.Stop()

	app := &App{
		webserver:      srv,
		config:         cfg,
		printerManager: mgr,
		oboxMgr:        nil,
	}

	appVariable := app.AppVariable()
	testutil.ExpectedEqual(t, appVariable.AppID, cfg.GetAppID())
	testutil.ExpectedEqual(t, appVariable.Server.IPAddress, util.LocalAddr(srv.Port, app.config.IsNetworkPrintingEnabled()))
	testutil.ExpectedEqual(t, util.GetPrinterUrl(srv.Port, app.config.IsNetworkPrintingEnabled(), "czpTTjEyMzQ1Ng"), fmt.Sprintf("%s/p/czpTTjEyMzQ1Ng", util.LocalAddr(srv.Port, app.config.IsNetworkPrintingEnabled())))
	testutil.ExpectedTrue(t, appVariable.Server.Running, "Expected ServerRunning to be true")
	testutil.ExpectedTrue(t, appVariable.System.OS != "", "Expected non-empty OS field in app variable")
	testutil.ExpectedTrue(t, appVariable.System.Arch != "", "Expected non-empty Arch field in app variable")
	testutil.ExpectedTrue(t, appVariable.System.Hostname != "", "Expected non-empty Hostname field in app variable")
}

func TestApp_ConfirmRemoveLANPrinter(t *testing.T) {
	const ip = "192.168.1.100"

	tests := []struct {
		name          string
		confirm       bool
		expectRemoved bool
	}{
		{name: "confirm removes printer", confirm: true, expectRemoved: true},
		{name: "cancel keeps printer", confirm: false, expectRemoved: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			cfg, err := config.NewManager()
			testutil.ExpectedNoError(t, err)
			testutil.ExpectedNoError(t, cfg.AddLanEposPrinter(ip))

			dialogs := &fakeDialogs{questionResp: tc.confirm}
			app := &App{config: cfg, printerManager: printer.NewManager(cfg), dialogs: dialogs}

			removed, err := app.ConfirmRemoveLANPrinter(ip)
			testutil.ExpectedNoError(t, err)
			testutil.ExpectedEqual(t, removed, tc.expectRemoved)

			expectedRemaining := 1
			if tc.expectRemoved {
				expectedRemaining = 0
			}
			testutil.ExpectedLen(t, cfg.GetLANPrinters(), expectedRemaining)

			testutil.ExpectedLen(t, dialogs.questions, 1)
			testutil.ExpectedContains(t, dialogs.questions[0].Message, ip)
		})
	}
}

func TestApp_ConfirmQuit(t *testing.T) {
	tests := []struct {
		name       string
		confirm    bool
		expectQuit bool
	}{
		{name: "confirm quits", confirm: true, expectQuit: true},
		{name: "cancel does not quit", confirm: false, expectQuit: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{dialogs: &fakeDialogs{questionResp: tc.confirm}}
			testutil.ExpectedEqual(t, app.confirmQuit(), tc.expectQuit)
		})
	}
}

func TestApp_DownloadLogs(t *testing.T) {
	initLogs := func(t *testing.T) string {
		t.Helper()
		t.Setenv("HOME", t.TempDir())
		logger.InitLogger()
		dir := logger.LogDirectory()
		testutil.ExpectedTrue(t, dir != "", "expected a log directory after InitLogger")
		return dir
	}

	newAppWithDialogs := func(t *testing.T, dialogs dialoger) *App {
		cfg, err := config.NewManager()
		testutil.ExpectedNoError(t, err)
		return &App{config: cfg, dialogs: dialogs}
	}

	t.Run("writes archive to the chosen path", func(t *testing.T) {
		initLogs(t)
		savePath := filepath.Join(t.TempDir(), "logs.zip")

		dialogs := &fakeDialogs{savePath: savePath}
		app := newAppWithDialogs(t, dialogs)

		app.DownloadLogs()

		info, err := os.Stat(savePath)
		testutil.ExpectedNoError(t, err)
		testutil.ExpectedTrue(t, info.Size() > 0, "expected a non-empty archive")

		testutil.ExpectedLen(t, dialogs.saves, 1)
		testutil.ExpectedContains(t, dialogs.saves[0].Filename, "obox-app-logs-")
		testutil.ExpectedLen(t, dialogs.errors, 0)
	})

	t.Run("cancelling the save dialog writes nothing", func(t *testing.T) {
		initLogs(t)

		dialogs := &fakeDialogs{savePath: ""}
		app := newAppWithDialogs(t, dialogs)

		app.DownloadLogs()

		testutil.ExpectedLen(t, dialogs.errors, 0)
	})

	t.Run("save dialog error is reported", func(t *testing.T) {
		initLogs(t)

		dialogs := &fakeDialogs{saveErr: errors.New("dialog unavailable")}
		app := newAppWithDialogs(t, dialogs)

		app.DownloadLogs()

		testutil.ExpectedLen(t, dialogs.errors, 1)
		testutil.ExpectedContains(t, dialogs.errors[0].Message, "dialog unavailable")
	})

	t.Run("zip failure is reported", func(t *testing.T) {
		initLogs(t)

		savePath := filepath.Join(t.TempDir(), "missing", "logs.zip")
		dialogs := &fakeDialogs{savePath: savePath}
		app := newAppWithDialogs(t, dialogs)

		app.DownloadLogs()

		testutil.ExpectedLen(t, dialogs.errors, 1)
		testutil.ExpectedContains(t, dialogs.errors[0].Message, "failed to create zip file")
	})
}

func TestApp_AutostartMethods(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager(cfg)
	srv := server.New(port, mgr, nil)
	defer srv.Stop()

	app := &App{
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
		events:         &fakeEvents{},
	}

	err = app.ToggleAutoStart()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, cfg.IsAutoStartEnabled())

	err = app.ToggleAutoStart()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedFalse(t, cfg.IsAutoStartEnabled())
}

func TestApp_NetworkPrintingEnabled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager(cfg)
	srv := server.New(port, mgr, nil)
	defer srv.Stop()

	events := &fakeEvents{}
	app := &App{
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
		events:         events,
	}

	testutil.ExpectedFalse(t, app.config.IsNetworkPrintingEnabled())
	testutil.ExpectedLen(t, events.getEvents(), 0)

	err = app.ToggleNetworkPrinting()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, app.config.IsNetworkPrintingEnabled())

	evts := events.getEvents()
	testutil.ExpectedLen(t, evts, 1)
	testutil.ExpectedEqual(t, evts[0].Name, "app:variables_changed")

	appVar, ok := evts[0].Data[0].(AppVariable)
	testutil.ExpectedTrue(t, ok, "expected payload to be AppVariable")
	testutil.ExpectedEqual(t, appVar.Server.IPAddress, util.LocalAddr(srv.Port, true))
	testutil.ExpectedTrue(t, appVar.Settings.NetworkPrintingEnabled)
}

func TestApp_GetTroubleshootInfo(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	_, err = cfg.ResolvePort()
	testutil.ExpectedNoError(t, err)

	app := &App{config: cfg}

	info := app.GetTroubleshootInfo()
	testutil.ExpectedTrue(t, info.Port > 0)
	testutil.ExpectedNotEqual(t, info.Subnet, "")
	testutil.ExpectedNotEqual(t, info.LocalIP, "")
}

func TestApp_NotifyAppVariablesChanged(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	port, err := cfg.ResolvePort()
	testutil.ExpectedNoError(t, err)

	mgr := printer.NewManager(cfg)
	srv := server.New(port, mgr, nil)
	defer srv.Stop()

	events := &fakeEvents{}
	app := &App{
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
		events:         events,
	}

	app.notifyAppVariablesChanged()

	evts := events.getEvents()
	testutil.ExpectedLen(t, evts, 1)
	testutil.ExpectedEqual(t, evts[0].Name, "app:variables_changed")

	appVar, ok := evts[0].Data[0].(AppVariable)
	testutil.ExpectedTrue(t, ok, "expected payload to be AppVariable")
	testutil.ExpectedEqual(t, appVar.AppID, cfg.GetAppID())
}

func TestApp_Shutdown_StopsOboxAndServer(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager(cfg)
	osMod := obox.NewManager(port, cfg, app_actions.OboxActionHandler(mgr))
	srv := server.New(port, mgr, osMod)

	app := &App{
		config:    cfg,
		webserver: srv,
		oboxMgr:   osMod,
	}

	app.shutdown()
	testutil.ExpectedFalse(t, srv.Running())

	appNil := &App{
		webserver: srv,
		oboxMgr:   nil,
	}
	appNil.shutdown()
}

func newTestConfig(t *testing.T) *config.Manager {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	return cfg
}

func newTestServerApp(t *testing.T) (*App, *config.Manager) {
	t.Helper()
	cfg := newTestConfig(t)
	port := testutil.GetFreePort(t)
	srv := server.New(port, nil, nil)
	t.Cleanup(func() { _ = srv.Stop() })

	app := &App{
		config:    cfg,
		webserver: srv,
		events:    &fakeEvents{},
	}
	return app, cfg
}

func TestApp_DebugModeEnabled(t *testing.T) {
	app, _ := newTestServerApp(t)

	testutil.ExpectedNoError(t, app.ToggleDebugMode())
	testutil.ExpectedTrue(t, app.AppVariable().Settings.DebugMode)
	testutil.ExpectedTrue(t, logger.IsDebugMode())
}

func TestApp_UpdateMethods(t *testing.T) {
	cfg := newTestConfig(t)
	app := &App{config: cfg}

	testutil.ExpectedEqual(t, cfg.LastSeenUpdate(), "")
	testutil.ExpectedNoError(t, app.MarkUpdateSeen("1.2.3"))
	testutil.ExpectedEqual(t, cfg.LastSeenUpdate(), "1.2.3")
}
