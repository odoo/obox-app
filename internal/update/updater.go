package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"obox-app/buildinfo"
	"obox-app/internal/logger"
)

const (
	checkCooldown        = 30 * time.Second
	defaultMaxNotesRunes = 500
)

type Updater struct {
	mu            sync.Mutex
	lastCheckTime time.Time
	lastestInfo   Info

	checksumAsset  *Asset
	appAsset       *Asset
	stagedPath     string
	stagedDir      string
	cancelDownload context.CancelFunc
}

var (
	instance *Updater
	once     sync.Once
)

func GetUpdaterInstance() *Updater {
	once.Do(func() {
		instance = &Updater{
			lastestInfo: Info{
				CurrentVersion: buildinfo.Version,
				State:          StateIdle,
			},
		}
	})
	return instance
}

func Check(ctx context.Context) (Info, error) {
	u := GetUpdaterInstance()
	u.mu.Lock()

	if u.lastestInfo.State.IsInProgress() {
		info := u.lastestInfo
		u.mu.Unlock()
		return info, ErrUpdateInProgress
	}

	// Use cached check if within cooldown
	if !u.lastCheckTime.IsZero() && time.Since(u.lastCheckTime) < checkCooldown {
		info := u.lastestInfo
		u.mu.Unlock()
		return info, nil
	}

	u.lastestInfo.State = StateChecking
	u.mu.Unlock()

	logger.Infof("Checking for updates (current version: %s)", buildinfo.Version)
	release, err := FetchLatestRelease(ctx)

	u.mu.Lock()
	defer u.mu.Unlock()

	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		logger.Warnf("Update check failed: %v", err)
		return u.lastestInfo, err
	}

	logger.Debugf("Fetched release info: tag=%s, assets_count=%d", release.TagName, len(release.Assets))

	isNewer, err := IsUpgrade(u.lastestInfo.CurrentVersion, release.TagName)
	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.LatestVersion = ""
		u.lastestInfo.Error = err.Error()
		logger.Errorf("Failed to compare versions: %v", err)
		return u.lastestInfo, err
	}

	logger.Debugf("Version comparison: current=%s, latest=%s, isUpgrade=%v", u.lastestInfo.CurrentVersion, release.TagName, isNewer)

	if !isNewer {
		u.lastestInfo.State = StateIdle
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
		u.lastCheckTime = time.Now()
		logger.Debugf("Application is up to date (%s)", u.lastestInfo.CurrentVersion)
		return u.lastestInfo, nil
	}

	// Skip automatic updates on macOS, as implementing and maintaining reliable
	// .app bundle replacement and code-signing support adds complexity
	if runtime.GOOS == "darwin" {
		u.lastestInfo.State = StateUpdateAvailable
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
		u.lastestInfo.Error = ""
		u.lastCheckTime = time.Now()
		logger.Infof("macOS: new version %s detected, manual download available", release.TagName)
		return u.lastestInfo, nil
	}

	asset, err := SelectAsset(release.Assets)
	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.LatestVersion = ""
		u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
		u.lastestInfo.Error = err.Error()
		logger.Errorf("Update %s available, but no asset: %v", release.TagName, err)
		return u.lastestInfo, err
	}

	u.appAsset = asset
	u.checksumAsset = FindChecksumAsset(release.Assets)

	u.lastestInfo.State = StateUpdateAvailable
	u.lastestInfo.LatestVersion = release.TagName
	u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
	u.lastestInfo.Error = ""
	u.lastCheckTime = time.Now()

	logger.Debugf("New update available: %s -> %s (asset: %s)", u.lastestInfo.CurrentVersion, release.TagName, asset.Name)
	return u.lastestInfo, nil
}

func Download(ctx context.Context, progress ProgressFunc) (string, error) {
	u := GetUpdaterInstance()
	u.mu.Lock()

	if runtime.GOOS == "darwin" {
		u.mu.Unlock()
		return "", errors.New("update from app is not supported on macOS; please download the new version manually")
	}

	if u.lastestInfo.State.IsInProgress() {
		u.mu.Unlock()
		return "", ErrUpdateInProgress
	}

	// If appAsset is missing, attempt to check for updates
	if u.appAsset == nil {
		u.mu.Unlock()
		info, err := Check(ctx)
		if err != nil {
			return "", err
		}
		if info.State != StateUpdateAvailable {
			return "", ErrUpdateNotReady
		}
		u.mu.Lock()
	}

	if u.lastestInfo.State != StateUpdateAvailable && u.lastestInfo.State != StateFailed {
		u.mu.Unlock()
		return "", ErrUpdateNotReady
	}

	// Clean up any stale staged directory before downloading fresh
	if u.stagedDir != "" {
		_ = os.RemoveAll(u.stagedDir)
		u.stagedDir = ""
		u.stagedPath = ""
	}

	asset := u.appAsset
	csAsset := u.checksumAsset

	downloadCtx, cancel := context.WithCancel(ctx)
	u.cancelDownload = cancel
	u.lastestInfo.State = StateDownloading
	u.lastestInfo.Error = ""
	u.mu.Unlock()

	logger.Debugf("Starting update download workflow for %s (url: %s)", asset.Name, asset.DownloadURL)

	defer func() {
		cancel()
		u.mu.Lock()
		u.cancelDownload = nil
		u.mu.Unlock()
	}()

	if err := ValidateDownloadURL(asset.DownloadURL); err != nil {
		u.mu.Lock()
		u.lastestInfo.State = StateUpdateAvailable
		u.lastestInfo.Error = err.Error()
		u.mu.Unlock()
		return "", err
	}

	// Fetch expected checksum before downloading - fail early if missing
	expectedSHA, err := FetchExpectedChecksum(downloadCtx, asset, csAsset)
	if err != nil || expectedSHA == "" {
		u.mu.Lock()
		if err != nil {
			u.lastestInfo.Error = err.Error()
		} else {
			u.lastestInfo.Error = fmt.Sprintf("no checksum available for asset %s", asset.Name)
		}
		u.lastestInfo.State = StateUpdateAvailable
		u.mu.Unlock()
		if err != nil {
			logger.Warnf("Could not fetch release checksums: %v", err)
			return "", fmt.Errorf("failed to fetch release checksums: %v", err)
		}
		return "", fmt.Errorf("no checksum available for asset %s", asset.Name)
	}
	logger.Debugf("Resolved expected SHA256 checksum for %s: %s", asset.Name, expectedSHA)

	result, err := DownloadAsset(downloadCtx, asset, expectedSHA, progress)
	u.mu.Lock()
	defer u.mu.Unlock()

	if err != nil {
		u.lastestInfo.State = StateUpdateAvailable
		if errors.Is(err, context.Canceled) {
			u.lastestInfo.Error = ""
			return "", err
		}
		u.lastestInfo.Error = err.Error()
		return "", err
	}

	// Check if context was cancelled while DownloadAsset was completing
	if downloadCtx.Err() != nil {
		if result != nil && result.TempDir != "" {
			_ = os.RemoveAll(result.TempDir)
		}
		u.lastestInfo.State = StateUpdateAvailable
		u.lastestInfo.Error = ""
		return "", downloadCtx.Err()
	}

	if u.stagedDir != "" && u.stagedDir != result.TempDir {
		logger.Debugf("Cleaning up previous staged directory: %s", u.stagedDir)
		_ = os.RemoveAll(u.stagedDir)
	}
	u.stagedPath = result.StagedPath
	u.stagedDir = result.TempDir
	u.lastestInfo.State = StateUpdateAvailable
	u.lastestInfo.Error = ""
	logger.Debugf("Update successfully downloaded and staged at: %s (size: %d bytes)", result.StagedPath, result.Size)
	return result.StagedPath, nil
}

// Apply installs the downloaded and verified update.
func (u *Updater) Apply() error {
	u.mu.Lock()
	if u.lastestInfo.State == StateInstalling {
		u.mu.Unlock()
		logger.Debugf("Apply aborted: install already in progress")
		return ErrUpdateInProgress
	}
	if u.stagedPath == "" {
		u.mu.Unlock()
		logger.Debugf("Apply aborted: no staged update path ready")
		return ErrUpdateNotReady
	}

	staged := u.stagedPath
	stagedDir := u.stagedDir
	u.lastestInfo.State = StateInstalling
	u.mu.Unlock()

	logger.Infof("Installing update from staged file %s", staged)
	if err := Apply(staged); err != nil {
		logger.Errorf("Apply failed: %v", err)
		u.mu.Lock()
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		if stagedDir != "" {
			logger.Debugf("Removing staged directory after apply failure: %s", stagedDir)
			_ = os.RemoveAll(stagedDir)
			u.stagedDir = ""
			u.stagedPath = ""
		}
		u.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrInstallFailed, err)
	}

	u.mu.Lock()
	u.lastestInfo.State = StateInstalled
	u.lastestInfo.Error = ""
	u.stagedDir = ""
	u.stagedPath = ""
	u.mu.Unlock()

	logger.Debugf("Apply successfully completed for staged file: %s", staged)
	return nil
}

// Reset returns the updater to StateIdle or StateUpdateAvailable if an update is available.
func (u *Updater) Reset() {
	u.mu.Lock()
	defer u.mu.Unlock()
	logger.Debugf("Resetting updater state (current state: %s, hasStagedPath: %v)", u.lastestInfo.State, u.stagedPath != "")
	if u.cancelDownload != nil {
		logger.Debugf("Cancelling active download context")
		u.cancelDownload()
		u.cancelDownload = nil
	}
	// Never delete the staged update directory or disrupt state while installation is in progress.
	if u.lastestInfo.State == StateInstalling {
		return
	}
	if u.appAsset != nil {
		u.lastestInfo.State = StateUpdateAvailable
	} else {
		u.lastestInfo.State = StateIdle
	}
	u.lastestInfo.Error = ""
	u.lastCheckTime = time.Time{}
}

func sanitizeNotes(notes string, maxRunes int) string {
	notes = strings.TrimSpace(strings.ReplaceAll(notes, "\r\n", "\n"))
	var b strings.Builder
	count := 0
	for _, r := range notes {
		if r != '\n' && r != '\t' && r < 32 {
			continue
		}
		if count == maxRunes {
			return b.String() + "…"
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}
