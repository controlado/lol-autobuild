package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubCheckerCompareVersions(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		latestVersion  string
		wantDraft      bool
		wantPreRelease bool
		wantAvailable  bool
		wantErr        error
	}{
		{"newer release", "0.1.0", "v1.2.0", false, false, true, nil},
		{"same release", "v0.2.0", "v0.2.0", false, false, false, nil},
		{"local newer", "0.3.0", "v0.2.0", false, false, false, nil},
		{"dev", "dev", "v0.2.0", false, false, false, ErrUnavailable},
		{"beta version", "v0.2.0", "v0.2.0-beta", false, false, false, ErrUnavailable},
		{"draft version", "0.1.0", "v0.2.0", true, false, false, ErrUnavailable},
		{"prerelease version", "0.1.0", "v0.2.0", false, true, false, ErrUnavailable},
		{"invalid remote version", "0.1.0", "v0.2.A", false, false, false, ErrUnavailable},
		{"remote version with only letters", "0.1.0", "unreachable", false, false, false, ErrUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wantDownloadURL := fmt.Sprintf("https://github.com/controlado/lol-autobuild/releases/tag/%s", tt.latestVersion)

			srv := newTestServer(t, githubRelease{
				TagName:    tt.latestVersion,
				HTMLURL:    wantDownloadURL,
				Draft:      tt.wantDraft,
				Prerelease: tt.wantPreRelease,
			})
			defer srv.Close()

			checker := NewGitHubChecker(Options{
				CurrentVersion:   tt.currentVersion,
				LatestReleaseURL: srv.URL,
			})

			result, err := checker.Check(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Check() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			if result.Available != tt.wantAvailable {
				t.Fatalf("Available = %v, want %v", result.Available, tt.wantAvailable)
			}
			if result.CurrentVersion != tt.currentVersion {
				t.Fatalf("CurrentVersion = %q, want %q", result.CurrentVersion, tt.currentVersion)
			}
			if result.LatestVersion != tt.latestVersion {
				t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, tt.latestVersion)
			}
			if result.DownloadURL != wantDownloadURL {
				t.Fatalf("DownloadURL = %q, want %q", result.DownloadURL, wantDownloadURL)
			}
		})
	}
}

func TestGitHubCheckerReturnsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer srv.Close()

	checker := NewGitHubChecker(Options{
		CurrentVersion:   "0.1.0",
		LatestReleaseURL: srv.URL,
	})

	_, err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	if errors.Is(err, ErrUnavailable) {
		t.Fatalf("Check() error = %v, should not be ErrUnavailable", err)
	}
}

func TestGitHubCheckerEmptyHTMLURL(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, githubRelease{TagName: "v1.0.0"})
	defer srv.Close()

	checker := NewGitHubChecker(Options{
		CurrentVersion:   "v1.0.0",
		LatestReleaseURL: srv.URL,
	})

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}

	if result.DownloadURL != defaultDownloadURL {
		t.Fatalf("DownloadURL = %q, want %q", result.DownloadURL, defaultDownloadURL)
	}
}

func TestNewGitHubCheckerBlankOptions(t *testing.T) {
	t.Parallel()

	checker := NewGitHubChecker(Options{})

	if checker.latestReleaseURL != defaultLatestReleaseURL {
		t.Fatalf("latestReleaseURL = %q, want %q", checker.latestReleaseURL, defaultLatestReleaseURL)
	}
	if checker.downloadFallbackURL != defaultDownloadURL {
		t.Fatalf("downloadFallbackURL = %q, want %q", checker.downloadFallbackURL, defaultDownloadURL)
	}
	if checker.httpClient == nil {
		t.Fatal("httpClient = nil, want http.Client")
	}
}

func newTestServer(t *testing.T, resp githubRelease) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatal("missing User-Agent")
		}

		_ = json.NewEncoder(w).Encode(resp)
	}))

}
