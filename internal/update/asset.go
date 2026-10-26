package update

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"obox-app/internal/logger"
	"runtime"
	"strings"
)

var allowedHosts = []string{"github.com", "api.github.com", "githubusercontent.com"}

func SelectAsset(assets []Asset) (*Asset, error) {
	logger.Debugf("Selecting update asset from %d candidates for %s/%s", len(assets), runtime.GOOS, runtime.GOARCH)
	if len(assets) == 0 {
		return nil, ErrNoCompatibleAsset
	}
	for i := range assets {
		a := &assets[i]
		if isAssetMatch(a.Name) {
			logger.Debugf("Matched update asset: %s (size=%d, url=%s)", a.Name, a.Size, a.DownloadURL)
			return a, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNoCompatibleAsset, runtime.GOOS+" "+runtime.GOARCH)
}

func isAssetMatch(filename string) bool {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".sha256") || strings.HasSuffix(lower, ".sha256sums") || strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".sig") {
		return false
	}

	expectedPrefix := fmt.Sprintf("obox-app-%s-%s", runtime.GOOS, runtime.GOARCH)
	return strings.HasPrefix(lower, expectedPrefix)
}

func FindChecksumAsset(assets []Asset) *Asset {
	for i := range assets {
		lower := strings.ToLower(assets[i].Name)
		if lower == "sha256sums" || lower == "checksums.txt" || strings.HasSuffix(lower, ".sha256") {
			logger.Debugf("Identified checksum asset: %s", assets[i].Name)
			return &assets[i]
		}
	}
	logger.Debugf("No dedicated checksum asset found in assets %v", assets)
	return nil
}

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

func FetchExpectedChecksum(ctx context.Context, targetAsset *Asset, csAsset *Asset) (string, error) {
	if sha := extractSHA256(targetAsset.Digest); sha != "" {
		logger.Debugf("Extracted SHA256 from target asset digest: %s", sha)
		return sha, nil
	}

	if csAsset == nil || csAsset.DownloadURL == "" {
		logger.Debugf("No checksum asset published for target asset: %s", targetAsset.Name)
		return "", nil // No checksum file published
	}

	sums, err := FetchChecksumFile(ctx, csAsset.DownloadURL)
	if err != nil {
		return "", err
	}

	if sha, ok := sums[targetAsset.Name]; ok {
		targetAsset.Digest = "sha256:" + sha
		logger.Debugf("Found expected checksum in checksum file for %s: %s", targetAsset.Name, sha)
		return sha, nil
	}

	logger.Debugf("Target asset %s not found in checksum file %s", targetAsset.Name, csAsset.Name)
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
