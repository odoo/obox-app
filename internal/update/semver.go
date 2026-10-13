package update

import (
	"fmt"
	"strconv"
	"strings"
)

type SemVer struct {
	Major int
	Minor int
	Patch int
}

func ParseSemVer(s string) (SemVer, error) {
	s = strings.TrimSpace(s)

	// Strip build metadata (+...) or suffix (-...)
	for _, sep := range []byte{'+', '-'} {
		if idx := strings.IndexByte(s, sep); idx >= 0 {
			s = s[:idx]
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf(
			"%w: version must start with MAJOR.MINOR.PATCH, got %q",
			ErrInvalidVersion, s,
		)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return SemVer{}, fmt.Errorf("%w: invalid version %q", ErrInvalidVersion, s)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return SemVer{}, fmt.Errorf("%w: invalid version %q", ErrInvalidVersion, s)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return SemVer{}, fmt.Errorf("%w: invalid version %q", ErrInvalidVersion, s)
	}

	return SemVer{Major: major, Minor: minor, Patch: patch}, nil
}

// compareVersions compares two version strings.
// Returns -1 if current < candidate, 0 if equal, and 1 if current > candidate.
func compareVersions(currentStr, candidateStr string) (int, error) {
	cand, err := ParseSemVer(candidateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid candidate version: %w", err)
	}

	if isDevVersion(currentStr) {
		return -1, nil // Dev/local builds are always considered older than a released candidate
	}

	curr, err := ParseSemVer(currentStr)
	if err != nil {
		return 0, fmt.Errorf("invalid current version: %w", err)
	}

	if curr.Major != cand.Major {
		return compareInt(curr.Major, cand.Major), nil
	}
	if curr.Minor != cand.Minor {
		return compareInt(curr.Minor, cand.Minor), nil
	}
	return compareInt(curr.Patch, cand.Patch), nil
}

// IsUpgrade checks if candidateStr is strictly newer than currentStr.
func IsUpgrade(currentStr, candidateStr string) (bool, error) {
	cmp, err := compareVersions(currentStr, candidateStr)
	if err != nil {
		return false, err
	}
	return cmp < 0, nil
}

func isDevVersion(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "" || v == "dev" || v == "local" || v == "unknown" || v == "none"
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
