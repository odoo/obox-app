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

	"obox-app/buildinfo"
	"obox-app/internal/logger"
)

var (
	// DefaultDownloadIdleTimeout defines how long a download can remain idle
	// without receiving data before timing out.
	DefaultDownloadIdleTimeout = 30 * time.Second

	// DefaultConnectTimeout defines how long to wait for a response before
	// timing out, including connection establishment and response headers.
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

func getUpdateCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		return os.TempDir()
	}

	cacheDir := filepath.Join(base, "OboxApp", "updates")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return os.TempDir()
	}

	return cacheDir
}

func DownloadAsset(ctx context.Context, asset *Asset, expectedSHA string, progress ProgressFunc) (*DownloadResult, error) {
	expectedSHA = strings.ToLower(strings.TrimSpace(expectedSHA))
	if expectedSHA == "" {
		return nil, fmt.Errorf("no checksum available for asset %s", asset.Name)
	}

	// Avoid a client-level timeout for the entire response body.
	// The request context and watchdog handle cancellation and timeouts.
	// Preserve the existing transport and redirect policy.
	dlClient := *client
	dlClient.Timeout = 0

	tempDir, err := os.MkdirTemp(getUpdateCacheDir(), "obox-app-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create update temporary directory: %w", err)
	}
	logger.Debugf("Created temporary update directory: %s", tempDir)

	// Remove incomplete downloads on every failure.
	keepTempDir := false
	defer func() {
		if !keepTempDir {
			logger.Debugf("Cleaning up temporary directory: %s", tempDir)
			_ = os.RemoveAll(tempDir)
		}
	}()

	tempFilePath := filepath.Join(tempDir, tempFileName)
	stagedFilePath := filepath.Join(tempDir, asset.Name)

	logger.Infof("Downloading update asset %q from %s", asset.Name, asset.DownloadURL)

	// A single goroutine owns the timer. Reset requests are sent through a
	// channel so the download goroutine never manipulates the timer directly.
	watchdogCtx, cancelWatchdog := context.WithCancel(ctx)
	defer cancelWatchdog()

	var stalled atomic.Bool

	resetTimer := make(chan time.Duration, 1)
	stopWatchdog := make(chan struct{})
	var stopWatchdogOnce sync.Once
	closeWatchdog := func() {
		stopWatchdogOnce.Do(func() {
			close(stopWatchdog)
		})
	}
	defer closeWatchdog()

	go func() {
		timer := time.NewTimer(DefaultConnectTimeout)
		defer timer.Stop()

		for {
			select {
			case <-stopWatchdog:
				return
			case <-watchdogCtx.Done():
				return
			case duration := <-resetTimer:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(duration)
			case <-timer.C:
				logger.Debugf("Download watchdog triggered: idle/connect timeout reached")
				stalled.Store(true)
				cancelWatchdog()
				return
			}
		}
	}()

	resetWatchdog := func(timeout time.Duration) {
		select {
		case resetTimer <- timeout:
		default:
			// Replace a pending reset with the most recent timeout.
			select {
			case <-resetTimer:
			default:
			}
			select {
			case resetTimer <- timeout:
			default:
			}
		}
	}

	req, err := http.NewRequestWithContext(watchdogCtx, http.MethodGet, asset.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build download request: %w", err)
	}
	req.Header.Set("User-Agent", requestUA+"/"+buildinfo.Version)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := dlClient.Do(req)
	if err != nil {
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
		return nil, fmt.Errorf("download failed: github returned HTTP %d", resp.StatusCode)
	}

	logger.Debugf("Download HTTP response received: status=%d, content-length=%d", resp.StatusCode, resp.ContentLength)

	// Start the idle timeout once the response headers have arrived.
	resetWatchdog(DefaultDownloadIdleTimeout)

	responseSize := resp.ContentLength
	total := responseSize
	if total <= 0 && asset.Size > 0 {
		total = asset.Size
	}

	file, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create staging file: %w", err)
	}
	logger.Debugf("Opened temporary staging file for writing: %s", tempFilePath)
	var fileClosed bool
	defer func() {
		if !fileClosed {
			_ = file.Close()
		}
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)
	buf := make([]byte, downloadBufferSize)

	var downloaded int64

	for {
		if stalled.Load() {
			return nil, ErrDownloadStalled
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			resetWatchdog(DefaultDownloadIdleTimeout)

			if _, err := writer.Write(buf[:n]); err != nil {
				return nil, fmt.Errorf("failed writing update data: %w", err)
			}

			downloaded += int64(n)

			if progress != nil {
				percent := 0
				if total > 0 {
					percent = min(int(downloaded*100/total), 100)
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
			if stalled.Load() {
				return nil, ErrDownloadStalled
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("download read error: %w", readErr)
		}
	}

	logger.Debugf("Download streaming finished: %d bytes read (expected: %d)", downloaded, total)

	// The watchdog is no longer needed after the response body is complete.
	closeWatchdog()
	logger.Debugf("Download watchdog stopped")

	if stalled.Load() {
		return nil, ErrDownloadStalled
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("failed to flush update to disk: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("failed to close update file: %w", err)
	}
	fileClosed = true
	logger.Debugf("Flushed and closed temporary file: %s", tempFilePath)

	if downloaded == 0 {
		return nil, fmt.Errorf("downloaded update file is empty")
	}

	if responseSize > 0 && downloaded != responseSize {
		return nil, fmt.Errorf("incomplete download: expected %d bytes, got %d", responseSize, downloaded)
	}
	if asset.Size > 0 && downloaded != asset.Size {
		return nil, fmt.Errorf("download size mismatch: expected %d bytes, got %d", asset.Size, downloaded)
	}

	computedHash := hex.EncodeToString(hasher.Sum(nil))
	logger.Debugf("Comparing SHA256: computed=%s, expected=%s", computedHash, expectedSHA)
	if computedHash != expectedSHA {
		logger.Errorf("Update checksum mismatch: got %s, want %s", computedHash, expectedSHA)
		return nil, fmt.Errorf("%w: got %s, expected %s", ErrChecksumMismatch, computedHash, expectedSHA)
	}

	logger.Infof("Update checksum verified: %s", computedHash)

	// Ensure executable permissions on Unix.
	if err := os.Chmod(tempFilePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to set update file permissions: %w", err)
	}
	logger.Debugf("Permissions set (0755) on %s", tempFilePath)

	// Stage the verified update only after all validations succeed.
	if err := os.Rename(tempFilePath, stagedFilePath); err != nil {
		return nil, fmt.Errorf("failed to stage update file: %w", err)
	}
	logger.Debugf("Staged binary renamed to: %s", stagedFilePath)

	if progress != nil && total > 0 {
		progress(Progress{
			State:      StateDownloading,
			Downloaded: downloaded,
			Total:      total,
			Percent:    100,
		})
	}

	keepTempDir = true

	logger.Infof("Update successfully staged at: %s", stagedFilePath)
	return &DownloadResult{
		StagedPath: stagedFilePath,
		TempDir:    tempDir,
		Size:       downloaded,
		SHA256:     computedHash,
	}, nil
}
