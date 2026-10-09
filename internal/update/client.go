package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"obox-app/buildinfo"
	"strings"
	"time"
)

const (
	defaultAPITimeout = 15 * time.Second
	maxJSONResponse   = 2 * 1024 * 1024 // 2 MB limit for API responses
)

var client = &http.Client{Timeout: defaultAPITimeout}

func FetchLatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", strings.TrimRight(getAPIBaseURL(), "/"), RepoOwner, RepoName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create update request: %w", err)
	}

	req.Header.Set("User-Agent", requestUA+"/"+buildinfo.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrNoReleases
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, fmt.Errorf("github api rate limit exceeded")
		}
		return nil, fmt.Errorf("github api access forbidden (status 403)")
	default:
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("github server error (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected github response (status %d)", resp.StatusCode)
	}

	var rel Release
	limitedReader := io.LimitReader(resp.Body, maxJSONResponse)
	if err := json.NewDecoder(limitedReader).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release json: %w", err)
	}

	rel.TagName = strings.TrimSpace(rel.TagName)
	if rel.TagName == "" {
		return nil, fmt.Errorf("%w: empty tag name in release", ErrInvalidVersion)
	}

	if _, err := ParseSemVer(rel.TagName); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidVersion, err)
	}

	return &rel, nil
}

// FetchChecksumFile downloads and parses a checksum file from the given URL.
func FetchChecksumFile(downloadURL string) (map[string]string, error) {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", requestUA+"/"+buildinfo.Version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch checksums: HTTP %d", resp.StatusCode)
	}

	sums, err := parseChecksums(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to parse checksums: %w", err)
	}

	return sums, nil
}
