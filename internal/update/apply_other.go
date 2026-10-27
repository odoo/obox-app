//go:build !linux && !windows

package update

import "errors"

func Apply(string) error {
	return errors.New("automatic update is not supported on this platform")
}

func PlatformNeedsQuitAfterApply() bool {
	return false
}
