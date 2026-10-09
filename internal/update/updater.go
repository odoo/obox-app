package update

import (
	"context"
	"fmt"
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

	latestRelease  *Release
	selectedAsset  *Asset
	stagedPath     string
	cancelDownload context.CancelFunc
}

var (
	instance *Updater
	once     sync.Once
)

// GetUpdaterInstance returns the singleton Updater instance.
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

	// Use cached check if within cooldown and not forced
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

	u.lastCheckTime = time.Now()

	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		logger.Warnf("Update check failed: %v", err)
		return u.lastestInfo, err
	}

	isNewer, err := IsUpgrade(u.lastestInfo.CurrentVersion, release.TagName)
	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Error = fmt.Sprintf("Invalid release version %q: %v", release.TagName, err)
		logger.Errorf("Failed to compare versions: %v", err)
		return u.lastestInfo, err
	}

	if !isNewer {
		u.lastestInfo.State = StateIdle
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
		logger.Debugf("Application is up to date (%s)", u.lastestInfo.CurrentVersion)
		return u.lastestInfo, nil
	}

	asset, err := selectAsset(release.Assets)
	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
		u.lastestInfo.Error = err.Error()
		logger.Errorf("Update %s available, but no asset: %v", release.TagName, err)
		return u.lastestInfo, err
	}

	u.latestRelease = release
	u.selectedAsset = asset

	u.lastestInfo.State = StateUpdateAvailable
	u.lastestInfo.LatestVersion = release.TagName
	u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
	u.lastestInfo.Error = ""

	logger.Debugf("New update available: %s -> %s (asset: %s)", u.lastestInfo.CurrentVersion, release.TagName, asset.Name)
	return u.lastestInfo, nil
}

// Download downloads and verifies the update asset.
func Download(ctx context.Context, progress ProgressFunc) (string, error) {
	u := GetUpdaterInstance()
	u.mu.Lock()

	if u.lastestInfo.State.IsInProgress() {
		return "", ErrUpdateInProgress
	}

	if u.selectedAsset == nil || u.latestRelease == nil {
		u.mu.Unlock()
		return "", ErrUpdateNotReady
	}

	asset := u.selectedAsset
	rel := u.latestRelease

	// Setup cancellable context
	downloadCtx, cancel := context.WithCancel(ctx)
	u.cancelDownload = cancel
	u.lastestInfo.State = StateDownloading
	u.mu.Unlock()

	// Validate download URL
	if err := ValidateDownloadURL(asset.DownloadURL); err != nil {
		u.mu.Lock()
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		u.mu.Unlock()
		return "", err
	}

	// Fetch expected checksum if available
	expectedSHA, err := FetchExpectedChecksum(asset, rel.Assets)
	if err != nil {
		u.mu.Lock()
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		u.mu.Unlock()
		logger.Warnf("Could not fetch release checksums: %v", err)
		return "", fmt.Errorf("failed to fetch release checksums: %v", err)
	}

	result, err := DownloadAsset(downloadCtx, asset, expectedSHA, progress)

	u.mu.Lock()
	defer u.mu.Unlock()

	u.cancelDownload = nil

	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		return "", err
	}

	u.lastestInfo.State = StateUpdateAvailable
	u.stagedPath = result.StagedPath
	u.lastestInfo.State = StateUpdateAvailable
	u.lastestInfo.Error = ""
	return result.StagedPath, nil
}

// Apply installs the downloaded and verified update.
func (u *Updater) Apply() error {
	u.mu.Lock()
	if u.lastestInfo.State == StateInstalling {
		u.mu.Unlock()
		return ErrUpdateInProgress
	}
	if u.stagedPath == "" {
		u.mu.Unlock()
		return ErrUpdateNotReady
	}

	staged := u.stagedPath
	u.lastestInfo.State = StateInstalling
	u.mu.Unlock()

	logger.Infof("Installing update from staged file %s", staged)
	if err := Apply(staged); err != nil {
		u.mu.Lock()
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		u.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrInstallFailed, err)
	}

	return nil
}

// Status returns the current Info summary.
func (u *Updater) Status() Info {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastestInfo
}

// Reset returns the updater to StateIdle.
func (u *Updater) Reset() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.cancelDownload != nil {
		u.cancelDownload()
		u.cancelDownload = nil
	}
	u.lastestInfo.State = StateIdle
	u.lastestInfo.Error = ""
}

func sanitizeNotes(notes string, maxRunes int) string {
	notes = strings.TrimSpace(notes)
	// Normalize CRLF to LF
	notes = strings.ReplaceAll(notes, "\r\n", "\n")
	// Strip control characters except newline and tab
	var b strings.Builder
	for _, r := range notes {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	s := b.String()
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return s
}
