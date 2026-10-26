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

	// Strip build metadata suffix (-...)
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("%w: version must start with MAJOR.MINOR.PATCH, got %q", ErrInvalidVersion, s)
	}

	nums := [3]int{}
	for i, part := range parts {
		val, err := strconv.Atoi(part)
		if err != nil || val < 0 {
			return SemVer{}, fmt.Errorf("%w: invalid version %q", ErrInvalidVersion, s)
		}
		nums[i] = val
	}

	return SemVer{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

func compareVersions(currentStr, candidateStr string) (int, error) {
	cand, err := ParseSemVer(candidateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid release version: %w", err)
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
	return strings.TrimSpace(strings.ToLower(v)) == "dev"
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
