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
	"sync"
	"testing"
	"time"

	"obox-app/internal/testutil"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

func getTestPlatformAssetName() string {
	switch runtime.GOOS {
	case "windows":
		return "obox-app-windows-amd64.exe"
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "obox-app-linux-arm64"
		}
		return "obox-app-linux-amd64"
	case "darwin":
		return "obox-app-darwin-amd64.pkg"
	default:
		return "obox-app"
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func setTestClient(t *testing.T, server *httptest.Server) {
	oldClient := client
	client = &http.Client{
		Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
			clone := req.Clone(req.Context())
			if server.TLS != nil {
				clone.URL.Scheme = "https"
			} else {
				clone.URL.Scheme = "http"
			}
			clone.URL.Host = server.Listener.Addr().String()
			clone.Host = server.Listener.Addr().String()
			return server.Client().Transport.RoundTrip(clone)
		}),
	}
	t.Cleanup(func() {
		client = oldClient
	})
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
						"browser_download_url": "%s/download/pkg"
					},
					{
						"name": "SHA256SUMS",
						"size": 100,
						"browser_download_url": "%s/download/sha256sums"
					}
				]
			}`, getTestPlatformAssetName(), len(binaryPayload), "https://"+r.Host, "https://"+r.Host)
		case "/download/pkg":
			w.Header().Set("Content-Length", fmt.Sprint(len(binaryPayload)))
			_, _ = w.Write(binaryPayload)
		case "/download/sha256sums":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "%s  %s\n", binaryHash, getTestPlatformAssetName())
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	setTestClient(t, server)

	var progressReported bool
	u := New().(*updaterImpl)
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.1.0"
	u.mu.Unlock()

	// 1. Initial State
	testutil.ExpectedEqual(t, u.lastestInfo.State, updater.StateIdle)

	// 2. Check for update
	info, err := u.Check(context.Background())
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, info.State, updater.StateAvailable)
	testutil.ExpectedEqual(t, info.LatestVersion, "1.2.0")
	testutil.ExpectedEqual(t, u.lastestInfo.State, updater.StateAvailable)

	// 3. Apply before download should fail
	err = u.Apply()
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))

	// 4. Download update
	staged, err := u.Download(context.Background(), func(p updater.Progress) {
		progressReported = true
	})
	testutil.ExpectedNoError(t, err)
	defer os.RemoveAll(u.stagedPath)

	testutil.ExpectedTrue(t, progressReported, "progress was not reported during download")
	testutil.ExpectedNotEqual(t, staged, "")
	testutil.ExpectedEqual(t, u.lastestInfo.State, updater.StateAvailable)

	// 5. Duplicate download while staged
	status := u.lastestInfo
	testutil.ExpectedEqual(t, status.State, updater.StateAvailable)
}

func TestUpdater_ConcurrencyProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name": "1.2.0", "draft": false, "prerelease": false, "assets": [{"name": "%s", "browser_download_url": "https://github.com/djip-odoo/obox-app/releases/download/v1.2.0/download"}]}`, getTestPlatformAssetName())
	}))
	defer server.Close()

	setTestClient(t, server)

	u := New().(*updaterImpl)
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.1.0"
	u.mu.Unlock()

	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err1 = u.Check(context.Background())
	}()
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // ensure first check has locked
		_, err2 = u.Check(context.Background())
	}()
	wg.Wait()

	testutil.ExpectedTrue(t, err1 == nil || errors.Is(err1, ErrUpdateInProgress))
	testutil.ExpectedTrue(t, err2 == nil || errors.Is(err2, ErrUpdateInProgress))
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
						"browser_download_url": "%s/download/pkg"
					},
					{
						"name": "SHA256SUMS",
						"size": 100,
						"browser_download_url": "%s/download/sha256sums"
					}
				]
			}`, getTestPlatformAssetName(), "https://"+r.Host, "https://"+r.Host)
		case "/download/sha256sums":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  %s\n", getTestPlatformAssetName())
		case "/download/pkg":
			w.Header().Set("Content-Length", "100000")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("start"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			close(downloadStarted)
			<-allowExit
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer func() {
		close(allowExit)
		server.Close()
	}()

	setTestClient(t, server)

	u := New().(*updaterImpl)
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.0.0"
	u.mu.Unlock()

	// 1. Check for update
	info, err := u.Check(context.Background())
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, info.State, updater.StateAvailable)

	// 2. Start download in a goroutine
	done := make(chan error, 1)
	go func() {
		_, dlErr := u.Download(context.Background(), nil)
		done <- dlErr
	}()

	// Wait for the server to receive the download request
	select {
	case <-downloadStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start in time")
	}

	// Wait briefly for state to become updater.StateDownloading
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if u.lastestInfo.State == updater.StateDownloading {
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

	// 5. State after Reset() should be updater.StateAvailable, no error, and staged files cleared
	finalStatus := u.lastestInfo
	testutil.ExpectedEqual(t, finalStatus.State, updater.StateAvailable)
	testutil.ExpectedEqual(t, finalStatus.Error, "")

	u.mu.Lock()
	stagedPath := u.stagedPath
	stagedDir := u.stagedDir
	u.mu.Unlock()
	testutil.ExpectedEqual(t, stagedPath, "")
	testutil.ExpectedEqual(t, stagedDir, "")
}

func TestUpdater_Check_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	setTestClient(t, server)

	u := New().(*updaterImpl)
	info, err := u.Check(context.Background())
	testutil.ExpectedError(t, err)
	testutil.ExpectedEqual(t, info.State, updater.StateError)
}

func TestUpdater_Download_FailureSetsStateError(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS does not support in-app download and update")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/djip-odoo/obox-app/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "1.2.0",
				"draft": false,
				"prerelease": false,
				"assets": [
					{
						"name": "%s",
						"size": 100,
						"browser_download_url": "%s/download/pkg"
					},
					{
						"name": "SHA256SUMS",
						"size": 100,
						"browser_download_url": "%s/download/sha256sums"
					}
				]
			}`, getTestPlatformAssetName(), "https://"+r.Host, "https://"+r.Host)
		case "/download/sha256sums":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  %s\n", getTestPlatformAssetName())
		case "/download/pkg":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	setTestClient(t, server)

	u := New().(*updaterImpl)
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.0.0"
	u.mu.Unlock()

	_, err := u.Check(context.Background())
	testutil.ExpectedNoError(t, err)

	_, err = u.Download(context.Background(), nil)
	testutil.ExpectedError(t, err)

	status := u.Info()
	testutil.ExpectedEqual(t, status.State, updater.StateError)
	testutil.ExpectedNotEqual(t, status.Error, "")
}
