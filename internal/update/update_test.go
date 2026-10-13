package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

const sampleReleaseFeedJSON = `{
  "id": 9999,
  "tag_name": "1.0.5",
  "name": "Release 1.0.5",
  "body": "[FIX] & network printing",
  "draft": false,
  "prerelease": false,
  "assets": [
    {
      "name": "obox-app-linux64",
      "size": 123,
      "browser_download_url": "http://127.0.0.1/download/linux64"
    }
  ]
}`

func TestCheck_VersionComparison(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleReleaseFeedJSON))
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
	u.lastestInfo.CurrentVersion = "dev"
	u.selectedAsset = nil
	u.latestRelease = nil
	u.stagedPath = ""
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()

	info, _ := Check(context.Background())
	if info.State != StateUpdateAvailable {
		t.Errorf("dev build should report update available: %+v", info)
	}
	if info.LatestVersion != "1.0.5" {
		t.Errorf("got latest version %q, want 1.0.5", info.LatestVersion)
	}

	// Same version -> not available
	u.Reset()
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.0.5"
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()

	info, _ = Check(context.Background())
	if info.State != StateIdle {
		t.Errorf("same version should report StateIdle: %+v", info)
	}
}

func TestCheck_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
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
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()

	info, _ := Check(context.Background())
	if info.State != StateFailed {
		t.Errorf("network failure should leave State failed: %+v", info)
	}
	if info.Error == "" {
		t.Error("expected an error message on network failure")
	}
}

func TestDownload_Helper(t *testing.T) {
	payload := []byte("update-binary-payload")
	hasher := sha256.New()
	hasher.Write(payload)
	payloadHash := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	asset := &Asset{
		Name:        "pkg.bin",
		DownloadURL: server.URL + "/pkg.bin",
	}

	var last int64
	res, err := DownloadAsset(context.Background(), asset, payloadHash, func(p Progress) {
		last = p.Downloaded
		if p.Downloaded > p.Total || p.Total != int64(len(payload)) {
			t.Errorf("bad progress d=%d total=%d", p.Downloaded, p.Total)
		}
	})
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	defer os.RemoveAll(res.TempDir)

	got, err := os.ReadFile(res.StagedPath)
	if err != nil {
		t.Fatalf("failed reading file: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("downloaded content mismatch: %q", got)
	}
	if last != int64(len(payload)) {
		t.Errorf("progress never reached total: got %d", last)
	}
}
