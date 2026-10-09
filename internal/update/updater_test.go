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
	"sync"
	"testing"
	"time"
)

func TestUpdater_Lifecycle(t *testing.T) {
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
						"name": "obox-app-linux64",
						"size": %d,
						"browser_download_url": "%s/download/app",
						"digest": "sha256:%s"
					},
					{
						"name": "obox-app-win64-installer",
						"size": %d,
						"browser_download_url": "%s/download/app",
						"digest": "sha256:%s"
					},
					{
						"name": "obox-app-macos",
						"size": %d,
						"browser_download_url": "%s/download/app",
						"digest": "sha256:%s"
					}
				]
			}`, len(binaryPayload), "https://"+r.Host, binaryHash,
				len(binaryPayload), "https://"+r.Host, binaryHash,
				len(binaryPayload), "https://"+r.Host, binaryHash)
		case "/download/app", "/download/linux64":
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
	u.Reset()
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.1.0"
	u.selectedAsset = nil
	u.latestRelease = nil
	u.stagedPath = ""
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	// 1. Initial State
	if u.Status().State != StateIdle {
		t.Errorf("initial state = %q, want %q", u.Status().State, StateIdle)
	}

	// 2. Check for update
	info, err := Check(context.Background())
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.State != StateUpdateAvailable {
		t.Errorf("expected update available, got: %+v", info)
	}
	if info.LatestVersion != "1.2.0" {
		t.Errorf("got latest version %q, want 1.2.0", info.LatestVersion)
	}
	if u.Status().State != StateUpdateAvailable {
		t.Errorf("state after check = %q, want %q", u.Status().State, StateUpdateAvailable)
	}

	// 3. Apply before download should fail
	if err := u.Apply(); !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("Apply before download should fail with ErrUpdateNotReady, got %v", err)
	}

	// 4. Download update
	var progressReported bool
	staged, err := Download(context.Background(), func(p Progress) {
		progressReported = true
	})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	defer os.RemoveAll(u.stagedPath)

	if !progressReported {
		t.Error("progress was not reported during download")
	}
	if staged == "" {
		t.Fatal("empty staged path returned")
	}
	if u.Status().State != StateUpdateAvailable {
		t.Errorf("state after download = %q, want %q", u.Status().State, StateUpdateAvailable)
	}

	// 5. Duplicate download while staged
	// Can re-download or check status
	status := u.Status()
	if status.State != StateUpdateAvailable {
		t.Errorf("expected UpdateAvailable status, got %s", status.State)
	}
}

func TestUpdater_ConcurrencyProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "1.2.0", "draft": false, "prerelease": false, "assets": [
			{"name": "obox-app-linux64", "browser_download_url": "http://127.0.0.1/download/linux64"},
			{"name": "obox-app-win64-installer", "browser_download_url": "http://127.0.0.1/download/win64"},
			{"name": "obox-app-macos", "browser_download_url": "http://127.0.0.1/download/macos"}
		]}`))
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
	u.selectedAsset = nil
	u.latestRelease = nil
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
	if err1 != nil && !errors.Is(err1, ErrUpdateInProgress) {
		t.Errorf("unexpected error on concurrent check 1: %v", err1)
	}
	if err2 != nil && !errors.Is(err2, ErrUpdateInProgress) {
		t.Errorf("unexpected error on concurrent check 2: %v", err2)
	}
}
