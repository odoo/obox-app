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
)

func TestDownloadAsset_Success(t *testing.T) {
	payload := []byte("binary-payload-data-for-update-test")
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
		Name:        "test-app",
		Size:        int64(len(payload)),
		DownloadURL: server.URL + "/test-app",
	}

	var progressCalled bool
	result, err := DownloadAsset(context.Background(), asset, payloadHash, func(p Progress) {
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

func TestDownloadAsset_ChecksumMismatch(t *testing.T) {
	payload := []byte("valid-payload")
	badHash := "0000000000000000000000000000000000000000000000000000000000000000"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	asset := &Asset{
		Name:        "test-app",
		DownloadURL: server.URL,
	}

	_, err := DownloadAsset(context.Background(), asset, badHash, nil)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrChecksumMismatch))
}

func TestDownloadAsset_MissingChecksum(t *testing.T) {
	asset := &Asset{
		Name:        "test-app",
		DownloadURL: "https://github.com/test",
	}

	_, err := DownloadAsset(context.Background(), asset, "", nil)
	testutil.ExpectedError(t, err)
}

func TestDownloadAsset_EmptyFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Empty response
	}))
	defer server.Close()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	asset := &Asset{
		Name:        "test-app",
		DownloadURL: server.URL,
	}

	dummySHA := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	_, err := DownloadAsset(context.Background(), asset, dummySHA, nil)
	testutil.ExpectedError(t, err)
}

func TestDownloadAsset_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write a little and sleep to allow cancellation
		_, _ = w.Write([]byte("start"))
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("end"))
	}))
	defer server.Close()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	asset := &Asset{
		Name:        "test-app",
		DownloadURL: server.URL,
	}

	dummySHA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err := DownloadAsset(ctx, asset, dummySHA, nil)
	testutil.ExpectedError(t, err)
}

func TestDownloadAsset_StallTimeout(t *testing.T) {
	origTimeout := DefaultDownloadIdleTimeout
	DefaultDownloadIdleTimeout = 100 * time.Millisecond
	defer func() { DefaultDownloadIdleTimeout = origTimeout }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("part1"))
		w.(http.Flusher).Flush()
		// Stall indefinitely without closing connection or sending further data
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	asset := &Asset{
		Name:        "test-app",
		DownloadURL: server.URL,
		Size:        1000,
	}

	dummySHA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err := DownloadAsset(context.Background(), asset, dummySHA, nil)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrDownloadStalled))
}

func TestGetUpdateCacheDir(t *testing.T) {
	dir := getUpdateCacheDir()
	testutil.ExpectedNotEqual(t, dir, "")

	info, err := os.Stat(dir)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, info.IsDir())
}
