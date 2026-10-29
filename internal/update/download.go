package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"obox-app/internal/logger"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
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

type watchdogWriter struct {
	w       io.Writer
	onWrite func()
}

func (ww *watchdogWriter) Write(b []byte) (int, error) {
	n, err := ww.w.Write(b)
	if n > 0 && ww.onWrite != nil {
		ww.onWrite()
	}
	return n, err
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

func downloadViaProvider(ctx context.Context, p *github.Provider, rel *updater.Release, asset *Asset, progress func(updater.Progress)) (*DownloadResult, error) {
	tempDir, err := os.MkdirTemp(getUpdateCacheDir(), "obox-app-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create update temporary directory: %w", err)
	}

	tempFilePath := filepath.Join(tempDir, tempFileName)
	stagedFilePath := filepath.Join(tempDir, asset.Name)

	f, err := os.Create(tempFilePath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)

	watchdogCtx, cancelWatchdog := context.WithCancel(ctx)
	defer cancelWatchdog()

	var stalled atomic.Bool
	var timerMu sync.Mutex
	timer := time.AfterFunc(DefaultConnectTimeout, func() {
		stalled.Store(true)
		logger.Warn("Download watchdog triggered: idle/connect timeout reached")
		cancelWatchdog()
	})
	defer func() {
		timerMu.Lock()
		timer.Stop()
		timerMu.Unlock()
	}()

	ww := &watchdogWriter{
		w: mw,
		onWrite: func() {
			timerMu.Lock()
			timer.Reset(DefaultDownloadIdleTimeout)
			timerMu.Unlock()
		},
	}

	err = p.Download(watchdogCtx, rel, ww, func(written, total int64) {
		if progress != nil {
			progress(updater.Progress{
				Written: written,
				Total:   total,
			})
		}
	})
	closeErr := f.Close()

	if stalled.Load() {
		_ = os.RemoveAll(tempDir)
		return nil, ErrDownloadStalled
	}
	if ctx.Err() != nil {
		_ = os.RemoveAll(tempDir)
		return nil, ctx.Err()
	}
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	if closeErr != nil {
		_ = os.RemoveAll(tempDir)
		return nil, closeErr
	}

	computedHash := hex.EncodeToString(hasher.Sum(nil))
	if rel.Verification != nil && len(rel.Verification.Digest) > 0 {
		expectedSHA := hex.EncodeToString(rel.Verification.Digest)
		if !strings.EqualFold(computedHash, expectedSHA) {
			_ = os.RemoveAll(tempDir)
			logger.Errorf("Update checksum mismatch: got %s, want %s", computedHash, expectedSHA)
			return nil, ErrChecksumMismatch
		}
	}

	if err := os.Rename(tempFilePath, stagedFilePath); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	if err := os.Chmod(stagedFilePath, 0755); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	fi, _ := os.Stat(stagedFilePath)
	var size int64
	if fi != nil {
		size = fi.Size()
	}

	return &DownloadResult{
		StagedPath: stagedFilePath,
		TempDir:    tempDir,
		Size:       size,
		SHA256:     computedHash,
	}, nil
}
