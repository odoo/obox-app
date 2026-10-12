//go:build windows

package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"epos-proxy/internal/testutil"

	"golang.org/x/sys/windows/svc"
)

func TestWindowsServiceExecuteLifecycle(t *testing.T) {
	app := NewApp()
	handler := &eposWindowsService{app: app}

	reqChan := make(chan svc.ChangeRequest)
	statusChan := make(chan svc.Status, 10)

	done := make(chan struct{})
	var svcSec bool
	var exitCode uint32

	go func() {
		svcSec, exitCode = handler.Execute([]string{"odoo-kiosk"}, reqChan, statusChan)
		close(done)
	}()

	// 1. First status must be StartPending
	select {
	case s := <-statusChan:
		testutil.ExpectedEqual(t, s.State, svc.StartPending)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for StartPending")
	}

	// 2. Next status must be Running
	select {
	case s := <-statusChan:
		testutil.ExpectedEqual(t, s.State, svc.Running)
		testutil.ExpectedEqual(t, s.Accepts, svc.AcceptStop|svc.AcceptShutdown)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Running")
	}

	// 3. Test HTTP request to 127.0.0.1:<port>
	port := app.config.GetPort()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	testutil.ExpectedNoError(t, err)
	if resp != nil {
		resp.Body.Close()
	}

	// 4. Send Stop command
	reqChan <- svc.ChangeRequest{Cmd: svc.Stop}

	// 5. Status must be StopPending
	select {
	case s := <-statusChan:
		testutil.ExpectedEqual(t, s.State, svc.StopPending)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for StopPending")
	}

	// 6. Execute must complete with code 0
	select {
	case <-done:
		testutil.ExpectedEqual(t, svcSec, false)
		testutil.ExpectedEqual(t, exitCode, uint32(0))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Execute to return")
	}
}
