package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"obox-app/internal/testutil"
)

func getTestPlatformAssetName() string {
	base := fmt.Sprintf("obox-app-%s-%s-1.2.0", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func TestUpdater_Lifecycle(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS does not support in-app download and update")
	}
	binaryPayload := []byte("new-version-payload")
	hasher := sha256.New()
	hasher.Write(binaryPayload)
	binaryHash := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/djip-odoo/obox-app/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "1.2.0",
				"body": "Bug fixes and improvements",
				"draft": false,
				"prerelease": false,
				"assets": [
					{
						"name": "%s",
						"size": %d,
						"browser_download_url": "%s/download/pkg",
						"digest": "sha256:%s"
					}
				]
			}`, getTestPlatformAssetName(), len(binaryPayload), "https://"+r.Host, binaryHash)
		case "/download/pkg":
			w.Header().Set("Content-Length", fmt.Sprint(len(binaryPayload)))
			_, _ = w.Write(binaryPayload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	oldHosts := allowedHosts
	allowedHosts = append(allowedHosts, "127.0.0.1")
	defer func() { allowedHosts = oldHosts }()

	u := GetUpdaterInstance()
	u.mu.Lock()
	u.appAsset = nil
	u.stagedPath = ""
	u.stagedDir = ""
	u.lastCheckTime = time.Time{}
	u.lastestInfo.CurrentVersion = "1.1.0"
	u.lastestInfo.LatestVersion = ""
	u.lastestInfo.State = StateIdle
	u.mu.Unlock()
	u.Reset()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	// 1. Initial State
	testutil.ExpectedEqual(t, u.lastestInfo.State, StateIdle)

	// 2. Check for update
	info, err := Check(context.Background())
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, info.State, StateUpdateAvailable)
	testutil.ExpectedEqual(t, info.LatestVersion, "1.2.0")
	testutil.ExpectedEqual(t, u.lastestInfo.State, StateUpdateAvailable)

	// 3. Apply before download should fail
	err = u.Apply()
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))

	// 4. Download update
	var progressReported bool
	staged, err := Download(context.Background(), func(p Progress) {
		progressReported = true
	})
	testutil.ExpectedNoError(t, err)
	defer os.RemoveAll(u.stagedPath)

	testutil.ExpectedTrue(t, progressReported, "progress was not reported during download")
	testutil.ExpectedNotEqual(t, staged, "")
	testutil.ExpectedEqual(t, u.lastestInfo.State, StateUpdateAvailable)

	// 5. Duplicate download while staged
	status := u.lastestInfo
	testutil.ExpectedEqual(t, status.State, StateUpdateAvailable)
}

func TestUpdater_ConcurrencyProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name": "1.2.0", "draft": false, "prerelease": false, "assets": [{"name": "%s", "browser_download_url": "https://github.com/djip-odoo/obox-app/releases/download/v1.2.0/download"}]}`, getTestPlatformAssetName())
	}))
	defer server.Close()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	u := GetUpdaterInstance()
	u.Reset()
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.1.0"
	u.appAsset = nil
	u.stagedPath = ""
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()

	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err1 = Check(context.Background())
	}()
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // ensure first check has locked
		_, err2 = Check(context.Background())
	}()
	wg.Wait()

	// At least one should succeed, or if a check was in flight the second gets ErrUpdateInProgress
	testutil.ExpectedTrue(t, err1 == nil || errors.Is(err1, ErrUpdateInProgress))
	testutil.ExpectedTrue(t, err2 == nil || errors.Is(err2, ErrUpdateInProgress))
}

func TestUpdater_RetryAfterFailureAndCancel(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS does not support in-app download and update")
	}
	u := GetUpdaterInstance()
	u.Reset()
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.0.0"
	u.appAsset = &Asset{
		Name:        getTestPlatformAssetName(),
		DownloadURL: "https://invalid-host-untrusted.example.com/asset",
	}
	u.lastestInfo.State = StateUpdateAvailable
	u.mu.Unlock()

	// Download should fail because of untrusted host
	_, err := Download(context.Background(), nil)
	testutil.ExpectedError(t, err)

	// State should remain StateUpdateAvailable with error set, enabling user to retry
	status := u.lastestInfo
	testutil.ExpectedEqual(t, status.State, StateUpdateAvailable)
	testutil.ExpectedNotEqual(t, status.Error, "")

	// Test Reset / Cancel behavior
	u.Reset()
	statusAfterReset := u.lastestInfo
	testutil.ExpectedEqual(t, statusAfterReset.State, StateUpdateAvailable)
	testutil.ExpectedEqual(t, statusAfterReset.Error, "")
	u.mu.Lock()
	testutil.ExpectedTrue(t, u.lastCheckTime.IsZero())
	u.mu.Unlock()
}

func TestUpdater_CancelMidDownload(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS does not support in-app download and update")
	}
	downloadStarted := make(chan struct{})
	allowExit := make(chan struct{})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/djip-odoo/obox-app/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "1.3.0",
				"draft": false,
				"prerelease": false,
				"assets": [
					{
						"name": "%s",
						"size": 100000,
						"browser_download_url": "%s/download/pkg",
						"digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
					}
				]
			}`, getTestPlatformAssetName(), "https://"+r.Host)
		case "/download/pkg":
			w.Header().Set("Content-Length", "100000")
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			close(downloadStarted)
			// Wait until test signals to exit or client disconnects
			select {
			case <-allowExit:
			case <-r.Context().Done():
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer func() {
		close(allowExit)
		server.Close()
	}()

	oldHosts := allowedHosts
	allowedHosts = append(allowedHosts, "127.0.0.1")
	defer func() { allowedHosts = oldHosts }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	u := GetUpdaterInstance()
	u.Reset()
	u.mu.Lock()
	u.appAsset = nil
	u.stagedPath = ""
	u.stagedDir = ""
	u.lastCheckTime = time.Time{}
	u.lastestInfo.CurrentVersion = "1.0.0"
	u.lastestInfo.LatestVersion = ""
	u.lastestInfo.State = StateIdle
	u.mu.Unlock()
	u.Reset()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	// 1. Check for update
	info, err := Check(context.Background())
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, info.State, StateUpdateAvailable)

	// 2. Start download in a goroutine
	done := make(chan error, 1)
	go func() {
		_, dlErr := Download(context.Background(), nil)
		done <- dlErr
	}()

	// Wait for the server to receive the download request
	select {
	case <-downloadStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start in time")
	}

	// Wait briefly for state to become StateDownloading
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if u.lastestInfo.State == StateDownloading {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 3. Cancel mid-download via Reset()
	u.Reset()

	// 4. Download goroutine must terminate promptly with error
	select {
	case dlErr := <-done:
		testutil.ExpectedError(t, dlErr)
	case <-time.After(3 * time.Second):
		t.Fatal("download did not abort in time after Reset()")
	}

	// 5. State after Reset() should be StateUpdateAvailable, no error, and staged files cleared
	finalStatus := u.lastestInfo
	testutil.ExpectedEqual(t, finalStatus.State, StateUpdateAvailable)
	testutil.ExpectedEqual(t, finalStatus.Error, "")

	u.mu.Lock()
	stagedPath := u.stagedPath
	stagedDir := u.stagedDir
	u.mu.Unlock()
	testutil.ExpectedEqual(t, stagedPath, "")
	testutil.ExpectedEqual(t, stagedDir, "")
}

func TestUpdater_Check_InvalidCurrentVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name": "1.2.0", "draft": false, "prerelease": false, "assets": []}`)
	}))
	defer server.Close()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	u := GetUpdaterInstance()
	u.Reset()
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "invalid-version"
	u.lastestInfo.State = StateIdle
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()

	info, err := Check(context.Background())
	testutil.ExpectedError(t, err)
	testutil.ExpectedEqual(t, info.State, StateFailed)
	testutil.ExpectedTrue(t, strings.Contains(info.Error, "invalid current version"), "expected error to mention current version, got: "+info.Error)
	testutil.ExpectedFalse(t, strings.Contains(info.Error, "invalid release version"), "error should not report invalid release version, got: "+info.Error)
}
