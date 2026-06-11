package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	repoOwner      = "poshpaws"
	repoName       = "jira-thing"
	requestTimeout = 5 * time.Second
)

// ReleaseInfo holds the latest release metadata.
type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// HTTPClient allows injecting a custom HTTP client for testing.
var HTTPClient = &http.Client{Timeout: requestTimeout}

// LatestRelease fetches the latest release tag from GitHub.
func LatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &info, nil
}

// CheckMessage compares current against latest and returns a user-facing message.
func CheckMessage(current string) string {
	release, err := LatestRelease()
	if err != nil {
		return fmt.Sprintf("could not check for updates: %v", err)
	}

	latest := normalise(release.TagName)
	curr := normalise(current)

	if curr == latest || current == "dev" {
		return fmt.Sprintf("you are up to date (%s)", release.TagName)
	}
	return fmt.Sprintf("update available: %s → %s\n  %s", current, release.TagName, release.HTMLURL)
}

// normalise strips a leading "v" for comparison.
func normalise(v string) string {
	return strings.TrimPrefix(v, "v")
}
