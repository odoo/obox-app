package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"obox-app/internal/testutil"
)

func TestDetermineLaunchMode(t *testing.T) {
	tests := []struct {
		name         string
		isService    bool
		forceKiosk   bool
		kioskEnabled bool
		expectedMode LaunchMode
	}{
		{
			name:         "Service mode takes priority over everything",
			isService:    true,
			forceKiosk:   false,
			kioskEnabled: false,
			expectedMode: ModeService,
		},
		{
			name:         "Service mode when kiosk enabled is also ModeService",
			isService:    true,
			forceKiosk:   true,
			kioskEnabled: true,
			expectedMode: ModeService,
		},
		{
			name:         "Interactive: kiosk.enabled missing/false -> ModeNormal",
			isService:    false,
			forceKiosk:   false,
			kioskEnabled: false,
			expectedMode: ModeNormal,
		},
		{
			name:         "Interactive: kiosk.enabled true -> ModeServer",
			isService:    false,
			forceKiosk:   false,
			kioskEnabled: true,
			expectedMode: ModeServer,
		},
		{
			name:         "Interactive: CLI flag --kiosk -> ModeServer",
			isService:    false,
			forceKiosk:   true,
			kioskEnabled: false,
			expectedMode: ModeServer,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode := determineLaunchMode(tc.isService, tc.forceKiosk, tc.kioskEnabled)
			testutil.ExpectedEqual(t, mode, tc.expectedMode)
		})
	}
}

func TestBackendHeadlessServiceLifecycle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Verify backend can start on 127.0.0.1, respond to HTTP requests,
	// and stop cleanly without Wails or WebView2 initialization.
	app := NewApp()

	port, err := app.startBackend()
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, port > 0, "Expected positive port")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = app.webserver.WaitReady(ctx)
	testutil.ExpectedNoError(t, err)

	// Verify server responds on 127.0.0.1
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	testutil.ExpectedNoError(t, err)
	defer resp.Body.Close()
	testutil.ExpectedTrue(t, resp.StatusCode > 0, "Expected valid HTTP status code")

	// Verify graceful shutdown
	err = app.webserver.Stop()
	testutil.ExpectedNoError(t, err)

	// Verify server is no longer accessible
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err = client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	testutil.ExpectedTrue(t, err != nil, "Server should be stopped")
}
