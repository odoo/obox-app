package update

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"runtime"
	"strings"
)

// selectAsset inspects the release's published assets and chooses the binary
// or installer matching the target operating system and CPU architecture.
func selectAsset(assets []Asset) (*Asset, error) {
	if len(assets) == 0 {
		return nil, ErrNoCompatibleAsset
	}
	for i := range assets {
		a := &assets[i]
		if isAssetMatch(a.Name) {
			return a, nil
		}
	}

	return nil, fmt.Errorf("%w", ErrNoCompatibleAsset)
}

func isAssetMatch(filename string) bool {
	lower := strings.ToLower(filename)

	switch runtime.GOOS {
	case "linux":
		return strings.HasPrefix(lower, assetNameLinux)

	case "windows":
		return strings.HasPrefix(lower, assetNameWindows)

	case "darwin":
		return strings.HasPrefix(lower, assetNameMacos)
	}
	return false
}

// FindChecksumAsset searches the assets for a checksum file like "SHA256SUMS" or "checksums.txt".
func FindChecksumAsset(assets []Asset) *Asset {
	for i := range assets {
		lower := strings.ToLower(assets[i].Name)
		if lower == "sha256sums" || lower == "checksums.txt" || strings.HasSuffix(lower, ".sha256") {
			return &assets[i]
		}
	}
	return nil
}

// parseChecksums parses a standard sha256sum file content (`<hash>  <filename>`)
// and returns a map from filename to expected lowercase sha256 hex string.
func parseChecksums(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := strings.ToLower(fields[0])
			name := strings.TrimPrefix(fields[1], "*") // binary mode prefix
			name = strings.TrimSpace(name)
			result[name] = hash
		}
	}
	return result, scanner.Err()
}

// FetchExpectedChecksum tries to get the expected SHA-256 for targetAsset:
// 1. Directly from the asset's "digest" field returned by the GitHub Release API (e.g. "sha256:...").
// 2. Otherwise downloads and parses any checksum asset in the release (e.g. SHA256SUMS).
func FetchExpectedChecksum(targetAsset *Asset, allAssets []Asset) (string, error) {
	if sha := extractSHA256(targetAsset.Digest); sha != "" {
		return sha, nil
	}

	csAsset := FindChecksumAsset(allAssets)
	if csAsset == nil || csAsset.DownloadURL == "" {
		return "", nil // No checksum file published
	}

	sums, err := FetchChecksumFile(csAsset.DownloadURL)
	if err != nil {
		return "", err
	}

	if sha, ok := sums[targetAsset.Name]; ok {
		targetAsset.Digest = "sha256:" + sha
		return sha, nil
	}

	return "", nil
}

func extractSHA256(digest string) string {
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "sha256") {
		return strings.ToLower(strings.TrimSpace(parts[1]))
	}
	if len(digest) == 64 {
		return strings.ToLower(strings.TrimSpace(digest))
	}
	return ""
}

var allowedHosts = []string{"github.com", "api.github.com"}

// ValidateDownloadURL verifies that the asset download URL points to a trusted GitHub domain.
func ValidateDownloadURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUntrustedSource, err)
	}

	if u.Scheme != "https" {
		return fmt.Errorf("%w: unsupported scheme %q", ErrUntrustedSource, u.Scheme)
	}

	host := strings.ToLower(u.Hostname())

	for _, a := range allowedHosts {
		if host == a || strings.HasSuffix(host, "."+a) {
			return nil
		}
	}

	return fmt.Errorf("%w: untrusted host %q", ErrUntrustedSource, host)
}
