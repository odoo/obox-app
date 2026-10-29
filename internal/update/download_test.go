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
	"testing"
	"time"

	"obox-app/internal/testutil"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func createTestProvider(serverURL string, client *http.Client) *github.Provider {
	p, _ := github.New(github.Config{
		Repository: "djip-odoo/obox-app",
		BaseURL:    serverURL,
		HTTPClient: client,
	})
	return p
}

func createTestRelease(filename, downloadURL string, size int64) *updater.Release {
	return &updater.Release{
		Artifact: updater.Artifact{
			Filename: filename,
			Size:     size,
		},
		Metadata: map[string]any{
			"github.asset.url": downloadURL,
		},
	}
}

func TestDownloadViaProvider_Success(t *testing.T) {
	payload := []byte("binary-payload-data-for-update-test")
	hasher := sha256.New()
	hasher.Write(payload)
	payloadHash := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	p := createTestProvider(server.URL, server.Client())
	rel := createTestRelease("test-app", server.URL+"/test-app", int64(len(payload)))
	rel.Verification = &updater.Verification{
		DigestAlgo: "sha256",
		Digest:     hasher.Sum(nil),
	}
	asset := &Asset{
		Name:        "test-app",
		Size:        int64(len(payload)),
		DownloadURL: server.URL + "/test-app",
	}

	var progressCalled bool
	result, err := downloadViaProvider(context.Background(), p, rel, asset, func(progress updater.Progress) {
		progressCalled = true
	})
	testutil.ExpectedNoError(t, err)
	defer os.RemoveAll(result.TempDir)

	testutil.ExpectedTrue(t, progressCalled, "progress callback was never called")
	testutil.ExpectedEqual(t, result.SHA256, payloadHash)
	testutil.ExpectedEqual(t, result.Size, int64(len(payload)))

	data, err := os.ReadFile(result.StagedPath)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedBytesEqual(t, data, payload)
}

func TestDownloadViaProvider_ChecksumMismatch(t *testing.T) {
	payload := []byte("valid-payload")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	p := createTestProvider(server.URL, server.Client())
	rel := createTestRelease("test-app", server.URL, int64(len(payload)))
	rel.Verification = &updater.Verification{
		DigestAlgo: "sha256",
		Digest:     make([]byte, 32), // bad hash
	}
	asset := &Asset{
		Name:        "test-app",
		DownloadURL: server.URL,
	}

	_, err := downloadViaProvider(context.Background(), p, rel, asset, nil)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrChecksumMismatch))
}

func TestDownloadViaProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("start"))
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("end"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := createTestProvider(server.URL, server.Client())
	rel := createTestRelease("test-app", server.URL, 100)
	asset := &Asset{
		Name:        "test-app",
		DownloadURL: server.URL,
	}

	_, err := downloadViaProvider(ctx, p, rel, asset, nil)
	testutil.ExpectedError(t, err)
}

func TestGetUpdateCacheDir(t *testing.T) {
	dir := getUpdateCacheDir()
	testutil.ExpectedNotEqual(t, dir, "")

	info, err := os.Stat(dir)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, info.IsDir())
}

func TestDownloadViaProvider_StallTimeout(t *testing.T) {
	origTimeout := DefaultDownloadIdleTimeout
	DefaultDownloadIdleTimeout = 100 * time.Millisecond
	defer func() { DefaultDownloadIdleTimeout = origTimeout }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("part1"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Stall indefinitely without closing connection or sending further data
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	p := createTestProvider(server.URL, server.Client())
	rel := createTestRelease("test-app", server.URL, 1000)
	asset := &Asset{
		Name:        "test-app",
		DownloadURL: server.URL,
	}

	_, err := downloadViaProvider(context.Background(), p, rel, asset, nil)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrDownloadStalled))
}
