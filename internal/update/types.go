package update

const (
	RepoOwner = "djip-odoo"
	RepoName  = "obox-app"

	assetNameLinux   = "obox-app-linux64"
	assetNameWindows = "obox-app-win64-installer"
	assetNameMacos   = "obox-app-macos"

	requestUA = "obox-app-updater"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

var (
	getAPIBaseURL  = func() string { return "https://api.github.com" }
	getFeedBaseURL = func() string { return "https://github.com" }
	// getAPIBaseURL  = func() string { return "https://test.com" }
	// getFeedBaseURL = func() string { return "https://test.com" }
)

// State represents the current lifecycle state of the updater.
type State string

const (
	StateIdle            State = "idle"
	StateUpdateAvailable State = "update_available"
	StateFailed          State = "failed"

	StateChecking    State = "checking"
	StateDownloading State = "downloading"
	StateInstalling  State = "installing"
)

func (s State) IsFinal() bool {
	switch s {
	case StateIdle, StateUpdateAvailable, StateFailed:
		return true
	default:
		return false
	}
}

func (s State) IsInProgress() bool {
	return !s.IsFinal()
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
	State          State  `json:"state"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Notes          string `json:"notes"`
	Error          string `json:"error,omitempty"`
	Seen           bool   `json:"seen"`
}

type Progress struct {
	State      State `json:"state,omitempty"`
	Downloaded int64 `json:"downloaded"`
	Total      int64 `json:"total"`
	Percent    int   `json:"percent"`
}

type ProgressFunc func(p Progress)
