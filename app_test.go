package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"obox-app/internal/app_actions"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/obox"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/testutil"
	"obox-app/internal/util"
)

func TestNewApp(t *testing.T) {
	app := NewApp()
	testutil.ExpectedNotNil(t, app)
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
		obox:           oboxMod,
	}

	status := app.CheckOdooStatus()
	testutil.ExpectedEqual(t, status.DBURL, "http://127.0.0.1:8069")
	testutil.ExpectedEqual(t, status.LanStatus, "connecting")
}

func TestApp_ConfirmDisconnectOdoo(t *testing.T) {
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
		obox:           oboxMod,
	}

	disconnected, err := app.ConfirmDisconnectOdoo()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, disconnected)
	testutil.ExpectedFalse(t, cfg.HasOdooCredentials())
}

func TestApp_ConfirmDisconnectOdoo_NilObox(t *testing.T) {
	app := &App{}
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
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedNoError(t, cfg.AddLanEposPrinter(ip))

	app := &App{config: cfg, printerManager: printer.NewManager(cfg)}

	removed, err := app.ConfirmRemoveLANPrinter(ip)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, removed)
	testutil.ExpectedLen(t, cfg.GetLANPrinters(), 0)
}

func TestApp_DownloadLogs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	logger.InitLogger()
	dir := logger.LogDirectory()
	testutil.ExpectedTrue(t, dir != "", "expected a log directory after InitLogger")

	savePath := filepath.Join(t.TempDir(), "logs.zip")
	err := util.ZipLogs(dir, savePath)
	testutil.ExpectedNoError(t, err)

	info, err := os.Stat(savePath)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, info.Size() > 0, "expected a non-empty archive")
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

	app := &App{
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
	}

	testutil.ExpectedFalse(t, app.config.IsNetworkPrintingEnabled())
	err = app.ToggleNetworkPrinting()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, app.config.IsNetworkPrintingEnabled())
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

	app := &App{
		config:         cfg,
		webserver:      srv,
		printerManager: mgr,
	}

	app.notifyAppVariablesChanged()
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
		obox:      osMod,
	}

	app.shutdown()
	testutil.ExpectedFalse(t, srv.Running())

	appNil := &App{
		webserver: srv,
		obox:      nil,
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
