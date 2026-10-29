package update

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

const (
	RepoOwner = "djip-odoo"
	RepoName  = "obox-app"
	requestUA = "obox-app-updater"
)

var (
	ErrInvalidVersion   = errors.New("invalid or malformed semantic version")
	ErrChecksumMismatch = errors.New("update file checksum verification failed")
	ErrUpdateInProgress = errors.New("an update operation is already in progress")
	ErrUpdateNotReady   = errors.New("no verified update is staged and ready to install")
	ErrInstallFailed    = errors.New("failed to install the update")
	ErrDownloadStalled  = errors.New("download connection lost or timed out")
)

func IsFinalState(s updater.State) bool {
	return !isInProgress(s)
}

func isInProgress(s updater.State) bool {
	switch s {
	case updater.StateChecking,
		updater.StateDownloading,
		updater.StateVerifying,
		updater.StateInstalling:
		return true
	default:
		return false
	}
}

// Asset represents a downloadable binary or installer in a release.
type Asset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
	Digest      string `json:"digest,omitempty"` // GitHub release asset digest, e.g. "sha256:abcd..."
}

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

// Info summarizes the current updater state and findings for the UI.
type Info struct {
	State          updater.State `json:"state"`
	CurrentVersion string        `json:"currentVersion"`
	LatestVersion  string        `json:"latestVersion"`
	Notes          string        `json:"notes"`
	Error          string        `json:"error,omitempty"`
	Seen           bool          `json:"seen"`
}
