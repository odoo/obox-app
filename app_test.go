package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"obox-app/buildinfo"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/testutil"
	"obox-app/internal/util"

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

func TestNewApp(t *testing.T) {
	app := NewApp()
	testutil.ExpectedNotNil(t, app)
	testutil.ExpectedNotNil(t, app.autoStart)
	testutil.ExpectedNotNil(t, app.printerManager)
}

func TestApp_AppVariableAndPrintersAndGetPrinterUrl(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	err = cfg.AddLanEposPrinter("192.168.1.100")
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	srv := server.New(port, mgr)
	defer srv.Stop()

	app := &App{
		webserver:      srv,
		config:         cfg,
		printerManager: mgr,
	}

	appVariable := app.AppVariable()
	testutil.ExpectedEqual(t, app.GetPrinterUrl("czpTTjEyMzQ1Ng"), fmt.Sprintf("%s:%d/p/czpTTjEyMzQ1Ng", util.GetLocalIP(app.IsNetworkPrintingEnabled()), port))
	testutil.ExpectedTrue(t, appVariable.ServerRunning, "Expected ServerRunning to be true")
	testutil.ExpectedTrue(t, appVariable.Os != "", "Expected non-empty Os field in app variable")
	testutil.ExpectedEqual(t, appVariable.Version, buildinfo.Version)
	testutil.ExpectedEqual(t, appVariable.BuildTime, buildinfo.BuildTime)
	testutil.ExpectedEqual(t, appVariable.Commit, buildinfo.Commit)
	testutil.ExpectedEqual(t, appVariable.IsDev, wailsruntime.Environment(app.ctx).BuildType == "dev")

	// Verify Printers() includes the configured LAN printer
	printers := app.Printers()
	foundLAN := false
	for _, p := range printers.Printers {
		if p.IsLAN && p.LANIp == "192.168.1.100" {
			foundLAN = true
			testutil.ExpectedEqual(t, p.Type, string(printer.TypeReceipt))
			testutil.ExpectedEqual(t, p.Name, "Network - 192.168.1.100")
			testutil.ExpectedEqual(t, p.Ip, fmt.Sprintf("%s:%d/p/%s", util.GetLocalIP(app.IsNetworkPrintingEnabled()), port, p.Id))
		}
	}
	testutil.ExpectedTrue(t, foundLAN, "Expected to find configured LAN printer in printer status")
}

func TestApp_AddLANPrinter(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	app := &App{config: cfg}

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
	app := &App{}

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
			app := &App{config: cfg, dialogs: dialogs}

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

	app := NewApp()

	// Enable autostart on linux creates desktop file
	err := app.EnableAutostart()
	testutil.ExpectedNoError(t, err)

	// Disable autostart
	_ = app.DisableAutostart()
}

func TestApp_NetworkPrintingEnabled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	srv := server.New(port, mgr)
	defer srv.Stop()

	app := &App{config: cfg, webserver: srv}

	// 1. Initial state (false)
	testutil.ExpectedFalse(t, app.IsNetworkPrintingEnabled())

	// 2. Enable network printing
	err = app.SetNetworkPrintingEnabled(true)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, app.IsNetworkPrintingEnabled())
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

func TestApp_DebugModeEnabled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	srv := server.New(port, mgr)
	defer srv.Stop()

	app := &App{
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
		events:         runtimeEvents{},
	}

	// Default state
	testutil.ExpectedFalse(t, cfg.IsDebugMode())
	testutil.ExpectedFalse(t, logger.IsDebugMode())

	// Enable debug mode via App
	err = app.SetSupportModeEnabled(true)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, cfg.IsDebugMode())
	testutil.ExpectedTrue(t, logger.IsDebugMode())
	testutil.ExpectedTrue(t, app.AppVariable().DebugMode)

	// Disable debug mode via App
	err = app.SetSupportModeEnabled(false)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedFalse(t, cfg.IsDebugMode())
	testutil.ExpectedFalse(t, logger.IsDebugMode())
	testutil.ExpectedFalse(t, app.AppVariable().DebugMode)
}

func TestApp_UpdateMethods(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	app := &App{config: cfg}

	// Test MarkUpdateSeen
	testutil.ExpectedEqual(t, cfg.LastSeenUpdate(), "")
	testutil.ExpectedNoError(t, app.MarkUpdateSeen("1.2.3"))
	testutil.ExpectedEqual(t, cfg.LastSeenUpdate(), "1.2.3")
}
