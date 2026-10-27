package update

import "errors"

var (
	ErrNoReleases        = errors.New("no published releases found")
	ErrNoCompatibleAsset = errors.New("no compatible asset found for platform")
	ErrInvalidVersion    = errors.New("invalid or malformed semantic version")
	ErrChecksumMismatch  = errors.New("update file checksum verification failed")
	ErrUpdateInProgress  = errors.New("an update operation is already in progress")
	ErrUpdateNotReady    = errors.New("no verified update is staged and ready to install")
	ErrInstallFailed     = errors.New("failed to install the update")
	ErrUntrustedSource   = errors.New("download URL is from an untrusted source")
	ErrDownloadStalled   = errors.New("download connection lost or timed out")
)
