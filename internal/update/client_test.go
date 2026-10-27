package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"obox-app/internal/testutil"
)

const sampleReleaseResponse = `{
  "id": 12345,
  "tag_name": "1.2.3",
  "name": "1.2.3 Stable",
  "body": "Fixed printing and networking issues.",
  "draft": false,
  "prerelease": false,
  "assets": [
    {
      "name": "obox-app-linux64-1.2.3",
      "size": 16000000,
      "browser_download_url": "https://github.com/djip-odoo/obox-app/releases/download/1.2.3/obox-app-linux64-1.2.3",
      "digest": "sha256:d5435a220dfde5650d9bc406313d4ab67c8b57424e79caa19fa1f1ac0c9d752e"
    }
  ]
}`

func mockServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldAPI, oldClient := getAPIBaseURL, client
	getAPIBaseURL = func() string { return server.URL }
	client = server.Client()

	t.Cleanup(func() {
		getAPIBaseURL, client = oldAPI, oldClient
	})
}

func TestFetchLatestRelease_Success(t *testing.T) {
	mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		testutil.ExpectedEqual(t, r.Header.Get("Accept"), "application/vnd.github+json")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleReleaseResponse))
	})

	rel, err := FetchLatestRelease(context.Background())
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, rel.TagName, "1.2.3")
	testutil.ExpectedLen(t, rel.Assets, 1)
	testutil.ExpectedEqual(t, rel.Assets[0].Name, "obox-app-linux64-1.2.3")
}

func TestFetchLatestRelease_Errors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		header     string
		body       string
		wantErr    error
		wantErrMsg string
	}{
		{
			name:    "not found",
			status:  http.StatusNotFound,
			wantErr: ErrNoReleases,
		},
		{
			name:       "rate limited",
			status:     http.StatusForbidden,
			header:     "0",
			wantErrMsg: "github api rate limit exceeded",
		},
		{
			name:       "malformed JSON",
			status:     http.StatusOK,
			body:       `{not-json`,
			wantErrMsg: "failed to parse release json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.header != "" {
					w.Header().Set("X-RateLimit-Remaining", tc.header)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			_, err := FetchLatestRelease(context.Background())
			if tc.wantErr != nil {
				testutil.ExpectedTrue(t, errors.Is(err, tc.wantErr))
			}
			if tc.wantErrMsg != "" {
				testutil.ExpectedErrorContains(t, err, tc.wantErrMsg)
			}
		})
	}
}
