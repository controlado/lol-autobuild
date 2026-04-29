package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLatestReleaseURL = "https://api.github.com/repos/controlado/lol-autobuild/releases/latest"
	defaultDownloadURL      = "https://github.com/controlado/lol-autobuild/releases/latest"
)

var ErrUnavailable = errors.New("update check unavailable")

type Result struct {
	CurrentVersion string
	LatestVersion  string
	DownloadURL    string
	Available      bool
}

type Options struct {
	CurrentVersion      string
	LatestReleaseURL    string
	DownloadFallbackURL string
	HTTPClient          *http.Client
}

type GitHubChecker struct {
	currentVersion      string
	latestReleaseURL    string
	downloadFallbackURL string
	httpClient          *http.Client
}

func NewGitHubChecker(opts Options) *GitHubChecker {
	if opts.LatestReleaseURL == "" {
		opts.LatestReleaseURL = defaultLatestReleaseURL
	}
	if opts.DownloadFallbackURL == "" {
		opts.DownloadFallbackURL = defaultDownloadURL
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &GitHubChecker{
		currentVersion:      strings.TrimSpace(opts.CurrentVersion),
		latestReleaseURL:    opts.LatestReleaseURL,
		downloadFallbackURL: opts.DownloadFallbackURL,
		httpClient:          opts.HTTPClient,
	}
}

func (c *GitHubChecker) CurrentVersion() string {
	return c.currentVersion
}

func (c *GitHubChecker) Check(ctx context.Context) (Result, error) {
	currentVersion, err := parseVersion(c.currentVersion)
	if err != nil {
		return Result{CurrentVersion: c.currentVersion}, err
	}

	release, err := c.fetchLatestRelease(ctx)
	if err != nil {
		return Result{CurrentVersion: c.currentVersion}, err
	}
	if release.Draft || release.Prerelease {
		return Result{CurrentVersion: c.currentVersion}, fmt.Errorf("%w: GitHub returned a draft or prerelease", ErrUnavailable)
	}

	latestVersion, err := parseVersion(release.TagName)
	if err != nil {
		return Result{CurrentVersion: c.currentVersion}, fmt.Errorf("parse GitHub release version: %w", err)
	}

	downloadURL := strings.TrimSpace(release.HTMLURL)
	if downloadURL == "" {
		downloadURL = c.downloadFallbackURL
	}

	return Result{
		CurrentVersion: c.currentVersion,
		LatestVersion:  strings.TrimSpace(release.TagName),
		DownloadURL:    downloadURL,
		Available:      latestVersion.compare(currentVersion) > 0,
	}, nil
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func (c *GitHubChecker) fetchLatestRelease(ctx context.Context) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.latestReleaseURL, nil)
	if err != nil {
		return githubRelease{}, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lol-autobuild")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return githubRelease{}, fmt.Errorf("GitHub latest release request failed %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("decode GitHub latest release: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, errors.New("GitHub latest release has no tag")
	}

	return release, nil
}

type version struct {
	major int
	minor int
	patch int
}

func parseVersion(raw string) (version, error) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "dev") {
		return version{}, fmt.Errorf("%w: cannot compare version %q", ErrUnavailable, raw)
	}

	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if strings.ContainsAny(s, "+-") {
		return version{}, fmt.Errorf("%w: cannot compare version %q", ErrUnavailable, raw)
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("%w: version %q must use major.minor.patch", ErrUnavailable, raw)
	}

	nums := make([]int, 3)
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return version{}, fmt.Errorf("%w: version %q must use numeric segments", ErrUnavailable, raw)
		}
		nums[i] = n
	}

	return version{
		major: nums[0],
		minor: nums[1],
		patch: nums[2],
	}, nil
}

func (v version) compare(other version) int {
	switch {
	case v.major != other.major:
		return v.major - other.major
	case v.minor != other.minor:
		return v.minor - other.minor
	default:
		return v.patch - other.patch
	}
}
