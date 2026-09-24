package main

import (
	"context"
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

	autostart "github.com/emersion/go-autostart"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// fakeDialogs is a dialoger that returns canned responses and records every
// invocation, so dialog-driven code paths can be tested without Wails.
type fakeDialogs struct {
	messageResult string
	messageErr    error
	savePath      string
	saveErr       error

	messages []wailsruntime.MessageDialogOptions
	saves    []wailsruntime.SaveDialogOptions
}

func (f *fakeDialogs) Message(_ context.Context, opts wailsruntime.MessageDialogOptions) (string, error) {
	f.messages = append(f.messages, opts)
	return f.messageResult, f.messageErr
}

func (f *fakeDialogs) SaveFile(_ context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
	f.saves = append(f.saves, opts)
	return f.savePath, f.saveErr
}

type emittedEvent struct {
	Name string
	Data []interface{}
}

// fakeEvents is an emitter that records every emitted event, so event-driven
// code paths can be tested without Wails.
type fakeEvents struct {
	mu      sync.Mutex
	emitted []emittedEvent
}

func (f *fakeEvents) Emit(_ context.Context, eventName string, optionalData ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emitted = append(f.emitted, emittedEvent{
		Name: eventName,
		Data: optionalData,
	})
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
	testutil.ExpectedNotNil(t, app.autoStart)
	testutil.ExpectedNotNil(t, app.events)
	testutil.ExpectedNotNil(t, app.config)
}

func TestApp_CheckOdooStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	cfg.Data.Port = testutil.GetFreePort(t)
	_ = cfg.SaveOdooCredentials("http://127.0.0.1:8069", "tok", "uuid-1")

	mgr := printer.NewManager(cfg)
	oboxMod := obox.NewManager(cfg.Data.Port, cfg, app_actions.OboxActionHandler(mgr))
	srv := server.New(cfg.Data.Port, mgr, oboxMod)
	defer srv.Stop()
	defer oboxMod.Stop()

	app := &App{
		webserver:      srv,
		config:         cfg,
		printerManager: mgr,
		obox:           oboxMod,
	}

	status := app.CheckOdooStatus()
	testutil.ExpectedEqual(t, status.DBURL, "http://127.0.0.1:8069")
	testutil.ExpectedEqual(t, status.LanStatus, "connecting")
}

func TestApp_ConfirmDisconnectOdoo(t *testing.T) {
	tests := []struct {
		name             string
		dialogResult     string
		dialogErr        error
		expectDisconnect bool
		expectErr        bool
	}{
		{name: "confirm disconnect", dialogResult: "Disconnect", expectDisconnect: true},
		{name: "cancel disconnect", dialogResult: "Cancel", expectDisconnect: false},
		{name: "dialog error", dialogErr: errors.New("dialog failed"), expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			cfg, err := config.NewManager()
			testutil.ExpectedNoError(t, err)
			cfg.Data.Port = testutil.GetFreePort(t)
			_ = cfg.SaveOdooCredentials("http://127.0.0.1:8069", "tok", "uuid-1")

			mgr := printer.NewManager(cfg)
			oboxMod := obox.NewManager(cfg.Data.Port, cfg, app_actions.OboxActionHandler(mgr))
			srv := server.New(cfg.Data.Port, mgr, oboxMod)
			defer srv.Stop()
			defer oboxMod.Stop()

			dialogs := &fakeDialogs{messageResult: tc.dialogResult, messageErr: tc.dialogErr}
			events := &fakeEvents{}
			app := &App{
				ctx:            context.Background(),
				webserver:      srv,
				config:         cfg,
				printerManager: mgr,
				obox:           oboxMod,
				dialogs:        dialogs,
				events:         events,
			}
			oboxMod.SetOnStatusChange(func(status obox.ConnectionStatus) {
				app.ev().Emit(app.ctx, "odoo:status_changed", status)
			})

			disconnected, err := app.ConfirmDisconnectOdoo()
			if tc.expectErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
			}
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
		obox:           nil,
		autoStart: &autostart.App{
			Name:        "obox-app",
			DisplayName: "Obox App",
			Exec:        []string{os.Args[0]},
		},
	}

	appVariable := app.AppVariable()
	testutil.ExpectedEqual(t, appVariable.AppID, cfg.GetAppID())
	testutil.ExpectedEqual(t, appVariable.IPAddress, util.LocalAddr(srv.Port, app.config.IsNetworkPrintingEnabled()))
	testutil.ExpectedEqual(t, util.GetPrinterUrl(srv.Port, app.config.IsNetworkPrintingEnabled(), "czpTTjEyMzQ1Ng"), fmt.Sprintf("%s/p/czpTTjEyMzQ1Ng", util.LocalAddr(srv.Port, app.config.IsNetworkPrintingEnabled())))
	testutil.ExpectedTrue(t, appVariable.ServerRunning, "Expected ServerRunning to be true")
	testutil.ExpectedTrue(t, appVariable.OS != "", "Expected non-empty Os field in app variable")

	// Verify Printers() includes the configured LAN printer
	printers := app.Printers()
	foundLAN := false
	for _, p := range printers.Printers {
		if p.IsLAN && p.LANIp == "192.168.1.100" {
			foundLAN = true
			testutil.ExpectedEqual(t, p.Type, string(printer.TypeReceipt))
			testutil.ExpectedEqual(t, p.Name, "Network - 192.168.1.100")
		}
	}
	testutil.ExpectedTrue(t, foundLAN, "Expected to find configured LAN printer in printer status")
}

func TestApp_AddLANPrinter(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	app := &App{config: cfg, printerManager: printer.NewManager(cfg)}

	// Invalid IP format.
	err = app.AddLANPrinter("not.an.ip")
	testutil.ExpectedError(t, err)

	// Empty IP.
	err = app.AddLANPrinter("  ")
	testutil.ExpectedError(t, err)

	// Unreachable printer.
	err = app.AddLANPrinter("127.0.0.254")
	testutil.ExpectedError(t, err)

	// Reachable printer.
	_, _, err = testutil.StartMockTCPServer(t)
	testutil.ExpectedNoError(t, err)

	err = app.AddLANPrinter("127.0.0.1")
	testutil.ExpectedNoError(t, err)

	printers := cfg.GetLANPrinters()
	testutil.ExpectedLen(t, printers, 1)
	testutil.ExpectedEqual(t, printers[0], "127.0.0.1")
}

func TestApp_CheckLANPrinterStatus(t *testing.T) {
	app := &App{printerManager: printer.NewManager(nil)}

	// 1. Unreachable (closed IP returns false)
	testutil.ExpectedFalse(t, app.CheckLANPrinterStatus("127.0.0.254"))

	// 2. Active listener using StartMockTCPServer
	_, _, err := testutil.StartMockTCPServer(t)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, app.CheckLANPrinterStatus("127.0.0.1"))
}

func TestApp_ConfirmRemoveLANPrinter(t *testing.T) {
	const ip = "192.168.1.100"

	tests := []struct {
		name          string
		dialogResult  string
		dialogErr     error
		expectRemoved bool
		expectErr     bool
	}{
		{name: "confirm removes printer", dialogResult: "Confirm", expectRemoved: true},
		{name: "linux yes button removes printer", dialogResult: "Yes", expectRemoved: true},
		{name: "cancel keeps printer", dialogResult: "Cancel"},
		{name: "dialog error keeps printer", dialogErr: errors.New("no display"), expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			cfg, err := config.NewManager()
			testutil.ExpectedNoError(t, err)
			testutil.ExpectedNoError(t, cfg.AddLanEposPrinter(ip))

			dialogs := &fakeDialogs{messageResult: tc.dialogResult, messageErr: tc.dialogErr}
			app := &App{config: cfg, printerManager: printer.NewManager(cfg), dialogs: dialogs}

			removed, err := app.ConfirmRemoveLANPrinter(ip)

			if tc.expectErr {
				testutil.ExpectedError(t, err)
			} else {
				testutil.ExpectedNoError(t, err)
			}
			testutil.ExpectedEqual(t, removed, tc.expectRemoved)

			// The printer must survive unless the user actually confirmed.
			expectedRemaining := 1
			if tc.expectRemoved {
				expectedRemaining = 0
			}
			testutil.ExpectedLen(t, cfg.GetLANPrinters(), expectedRemaining)

			// Exactly one confirmation dialog is shown, and it names the printer.
			testutil.ExpectedLen(t, dialogs.messages, 1)
			testutil.ExpectedContains(t, dialogs.messages[0].Message, ip)
		})
	}
}

func TestApp_ConfirmQuit(t *testing.T) {
	tests := []struct {
		name         string
		dialogResult string
		dialogErr    error
		expectQuit   bool
	}{
		{name: "quit button confirms", dialogResult: "Quit", expectQuit: true},
		{name: "linux yes button confirms", dialogResult: "Yes", expectQuit: true},
		{name: "cancel does not quit", dialogResult: "Cancel"},
		{name: "dialog error does not quit", dialogErr: errors.New("no display")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{dialogs: &fakeDialogs{messageResult: tc.dialogResult, messageErr: tc.dialogErr}}
			testutil.ExpectedEqual(t, app.ConfirmQuit(), tc.expectQuit)
		})
	}
}

func TestApp_DownloadLogs(t *testing.T) {
	// initLogs points the logger at a temporary directory containing one log file, and returns that directory.
	initLogs := func(t *testing.T) string {
		t.Helper()
		t.Setenv("HOME", t.TempDir())
		logger.InitLogger()
		dir := logger.LogDirectory()
		testutil.ExpectedTrue(t, dir != "", "expected a log directory after InitLogger")
		return dir
	}

	t.Run("writes archive to the chosen path", func(t *testing.T) {
		initLogs(t)
		savePath := filepath.Join(t.TempDir(), "logs.zip")

		dialogs := &fakeDialogs{savePath: savePath}
		app := &App{dialogs: dialogs}

		app.DownloadLogs()

		info, err := os.Stat(savePath)
		testutil.ExpectedNoError(t, err)
		testutil.ExpectedTrue(t, info.Size() > 0, "expected a non-empty archive")

		testutil.ExpectedLen(t, dialogs.saves, 1)
		testutil.ExpectedContains(t, dialogs.saves[0].DefaultFilename, "obox-app-logs-")
		testutil.ExpectedLen(t, dialogs.messages, 0)
	})

	t.Run("cancelling the save dialog writes nothing", func(t *testing.T) {
		initLogs(t)

		// Wails returns an empty path when the user dismisses the dialog.
		dialogs := &fakeDialogs{savePath: ""}
		app := &App{dialogs: dialogs}

		app.DownloadLogs()

		// No archive attempted and, crucially, no error surfaced to the user.
		testutil.ExpectedLen(t, dialogs.messages, 0)
	})

	t.Run("save dialog error is reported", func(t *testing.T) {
		initLogs(t)

		dialogs := &fakeDialogs{saveErr: errors.New("dialog unavailable")}
		app := &App{dialogs: dialogs}

		app.DownloadLogs()

		testutil.ExpectedLen(t, dialogs.messages, 1)
		testutil.ExpectedEqual(t, dialogs.messages[0].Type, wailsruntime.ErrorDialog)
		testutil.ExpectedContains(t, dialogs.messages[0].Message, "dialog unavailable")
	})

	t.Run("zip failure is reported", func(t *testing.T) {
		initLogs(t)

		// Parent directory does not exist, so creating the archive fails.
		savePath := filepath.Join(t.TempDir(), "missing", "logs.zip")
		dialogs := &fakeDialogs{savePath: savePath}
		app := &App{dialogs: dialogs}

		app.DownloadLogs()

		testutil.ExpectedLen(t, dialogs.messages, 1)
		testutil.ExpectedEqual(t, dialogs.messages[0].Type, wailsruntime.ErrorDialog)
		testutil.ExpectedContains(t, dialogs.messages[0].Message, "failed to create zip file")
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
		autoStart: &autostart.App{
			Name:        "obox-app",
			DisplayName: "Obox App",
			Exec:        []string{os.Args[0]},
		},
	}

	// Enable autostart on linux creates desktop file
	err = app.ToggleAutoStart()
	testutil.ExpectedNoError(t, err)

	// Disable autostart
	err = app.ToggleAutoStart()
	testutil.ExpectedNoError(t, err)
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
		ctx:            context.Background(),
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
		events:         events,
		autoStart: &autostart.App{
			Name:        "obox-app",
			DisplayName: "Obox App",
			Exec:        []string{os.Args[0]},
		},
	}

	// 1. Initial state (false)
	testutil.ExpectedFalse(t, app.config.IsNetworkPrintingEnabled())
	testutil.ExpectedLen(t, events.getEvents(), 0)

	// 2. Toggle network printing to enable
	err = app.ToggleNetworkPrinting()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, app.config.IsNetworkPrintingEnabled())

	// Verify single consolidated event was emitted: app:variables_changed
	evts := events.getEvents()
	testutil.ExpectedLen(t, evts, 1)
	testutil.ExpectedEqual(t, evts[0].Name, "app:variables_changed")

	appVar, ok := evts[0].Data[0].(AppVariable)
	testutil.ExpectedTrue(t, ok, "expected payload to be AppVariable")
	testutil.ExpectedEqual(t, appVar.IPAddress, util.LocalAddr(srv.Port, true))
	testutil.ExpectedTrue(t, appVar.NetworkPrintingEnabled)
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
		ctx:            context.Background(),
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
		events:         events,
		autoStart: &autostart.App{
			Name:        "obox-app",
			DisplayName: "Obox App",
			Exec:        []string{os.Args[0]},
		},
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
		ctx:       context.Background(),
		config:    cfg,
		webserver: srv,
		obox:      osMod,
	}

	app.shutdown(context.Background())
	testutil.ExpectedFalse(t, srv.Running())

	// Calling shutdown with nil obox should also be safe
	appNil := &App{
		ctx:       context.Background(),
		webserver: srv,
		obox:      nil,
	}
	appNil.shutdown(context.Background())
}
