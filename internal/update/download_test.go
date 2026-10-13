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
	if err != nil {
		t.Fatalf("DownloadAsset error: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	if !progressCalled {
		t.Error("progress callback was never called")
	}
	if result.SHA256 != payloadHash {
		t.Errorf("got SHA %s, want %s", result.SHA256, payloadHash)
	}
	if result.Size != int64(len(payload)) {
		t.Errorf("got size %d, want %d", result.Size, len(payload))
	}

	data, err := os.ReadFile(result.StagedPath)
	if err != nil {
		t.Fatalf("failed reading staged file: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("content mismatch: got %q, want %q", data, payload)
	}
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
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("got error %v, want ErrChecksumMismatch", err)
	}
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

	_, err := DownloadAsset(context.Background(), asset, "", nil)
	if err == nil {
		t.Fatal("expected error for empty download, got nil")
	}
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

	_, err := DownloadAsset(ctx, asset, "", nil)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
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

	_, err := DownloadAsset(context.Background(), asset, "", nil)
	if err == nil {
		t.Fatal("expected stall timeout error, got nil")
	}
	if !errors.Is(err, ErrDownloadStalled) {
		t.Errorf("got error %v, want ErrDownloadStalled", err)
	}
}
