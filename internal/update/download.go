package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"obox-app/internal/logger"
)

var (
	// DefaultDownloadIdleTimeout defines how long a download can remain idle with no incoming data before timing out.
	DefaultDownloadIdleTimeout = 20 * time.Second
	// DefaultConnectTimeout defines the timeout for establishing connection and receiving response headers.
	DefaultConnectTimeout = 30 * time.Second
)

const (
	downloadBufferSize = 64 * 1024
	tempFileName       = "update.tmp"
)

// DownloadResult contains details about a staged update.
type DownloadResult struct {
	StagedPath string
	TempDir    string
	Size       int64
	SHA256     string
}

func DownloadAsset(ctx context.Context, asset *Asset, expectedSHA string, progress ProgressFunc) (*DownloadResult, error) {
	// Ensure we do not inherit a short client-level whole-body timeout for downloading large binaries.
	// We preserve the transport (for proxy/custom TLS/mocking in tests) while setting Timeout = 0,
	// allowing ctx to manage cancellation and deadlines.
	dlClient := &http.Client{Timeout: 0}
	if client != nil {
		dlClient.Transport = client.Transport
		dlClient.CheckRedirect = client.CheckRedirect
		dlClient.Jar = client.Jar
	}
	client = dlClient

	tempDir, err := os.MkdirTemp("", "obox-app-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create update temporary directory: %w", err)
	}

	tempFilePath := filepath.Join(tempDir, tempFileName)
	stagedFilePath := filepath.Join(tempDir, asset.Name)

	cleanup := func() {
		_ = os.Remove(tempFilePath)
		_ = os.Remove(stagedFilePath)
		_ = os.RemoveAll(tempDir)
	}

	logger.Infof("Downloading update asset %q from %s", asset.Name, asset.DownloadURL)

	// Setup watchdog to detect dead connections / network drops mid-download
	watchdogCtx, cancelWatchdog := context.WithCancel(ctx)
	defer cancelWatchdog()

	var (
		stalled atomic.Bool
		timerMu sync.Mutex
	)

	timer := time.NewTimer(DefaultConnectTimeout)
	defer timer.Stop()

	stopWatchdog := make(chan struct{})
	defer close(stopWatchdog)

	go func() {
		for {
			select {
			case <-stopWatchdog:
				return
			case <-watchdogCtx.Done():
				return
			case <-timer.C:
				stalled.Store(true)
				cancelWatchdog()
				return
			}
		}
	}()

	resetTimer := func(d time.Duration) {
		timerMu.Lock()
		defer timerMu.Unlock()
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(d)
	}

	req, err := http.NewRequestWithContext(watchdogCtx, http.MethodGet, asset.DownloadURL, nil)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to build download request: %w", err)
	}
	req.Header.Set("User-Agent", requestUA+"/"+Version)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		cleanup()
		if stalled.Load() {
			return nil, ErrDownloadStalled
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("download connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return nil, fmt.Errorf("download failed: github returned HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 && asset.Size > 0 {
		total = asset.Size
	}

	file, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create staging file: %w", err)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)

	buf := make([]byte, downloadBufferSize)
	var downloaded int64

	// Reset watchdog for body streaming: if no data arrives for DefaultDownloadIdleTimeout, abort
	resetTimer(DefaultDownloadIdleTimeout)

	for {
		if stalled.Load() {
			_ = file.Close()
			cleanup()
			return nil, ErrDownloadStalled
		}
		if ctx.Err() != nil {
			_ = file.Close()
			cleanup()
			return nil, ctx.Err()
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			resetTimer(DefaultDownloadIdleTimeout)
			if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
				_ = file.Close()
				cleanup()
				return nil, fmt.Errorf("failed writing update data: %w", writeErr)
			}
			downloaded += int64(n)
			if progress != nil {
				percent := 0
				if total > 0 {
					percent = int((downloaded * 100) / total)
					if percent > 100 {
						percent = 100
					}
				}
				progress(Progress{
					State:      StateDownloading,
					Downloaded: downloaded,
					Total:      total,
					Percent:    percent,
				})
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			_ = file.Close()
			cleanup()
			if stalled.Load() {
				return nil, ErrDownloadStalled
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("download read error: %w", readErr)
		}
	}

	if err := file.Sync(); err != nil {
		_ = file.Close()
		cleanup()
		return nil, fmt.Errorf("failed to flush update to disk: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to close update file: %w", err)
	}

	// Validate non-empty download
	if downloaded == 0 {
		cleanup()
		return nil, fmt.Errorf("downloaded update file is empty")
	}

	// Validate expected size if known
	if total > 0 && downloaded != total {
		cleanup()
		return nil, fmt.Errorf("incomplete download: expected %d bytes, got %d", total, downloaded)
	}

	computedHash := hex.EncodeToString(hasher.Sum(nil))
	expectedSHA = strings.ToLower(strings.TrimSpace(expectedSHA))

	// Verify SHA-256 if expected hash is available
	if expectedSHA != "" {
		if computedHash != expectedSHA {
			cleanup()
			logger.Errorf("Update checksum mismatch: got %s, want %s", computedHash, expectedSHA)
			return nil, fmt.Errorf("%w: got %s, expected %s", ErrChecksumMismatch, computedHash, expectedSHA)
		}
		logger.Infof("Update checksum verified: %s", computedHash)
	} else {
		logger.Warnf("No checksum available for asset %s; verified size (%d bytes) only", asset.Name, downloaded)
		cleanup()
		return nil, fmt.Errorf("no checksum available for asset %s", asset.Name)
	}

	// Ensure executable permissions (Unix)
	_ = os.Chmod(tempFilePath, 0o755)

	// Atomically rename update.tmp to the staged file
	if err := os.Rename(tempFilePath, stagedFilePath); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to stage update file: %w", err)
	}

	if progress != nil && total > 0 {
		progress(Progress{
			State:      StateDownloading,
			Downloaded: total,
			Total:      total,
			Percent:    100,
		})
	}

	logger.Infof("Update successfully staged at: %s", stagedFilePath)
	return &DownloadResult{
		StagedPath: stagedFilePath,
		TempDir:    tempDir,
		Size:       downloaded,
		SHA256:     computedHash,
	}, nil
}
