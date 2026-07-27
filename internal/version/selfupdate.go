package version

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"

	update "github.com/inconshreveable/go-update"
)

// downloadFn allows tests to stub out the asset download.
var downloadFn = downloadAsset

// applyFn allows tests to stub out the binary swap (it otherwise overwrites os.Executable()).
var applyFn = func(r io.Reader) error {
	return update.Apply(r, update.Options{})
}

// SelfUpdate replaces the running binary with the latest GitHub release, if newer.
// Returns a human-readable result message. A "dev" build cannot be compared
// against a semver release, so it is rejected before making any network call.
func SelfUpdate(ctx context.Context, current string) (string, error) {
	if current == "dev" {
		return "", fmt.Errorf("cannot self-update a dev build; download a tagged release from GitHub")
	}

	release, err := LatestRelease()
	if err != nil {
		return "", fmt.Errorf("checking latest release: %w", err)
	}
	if normalise(release.TagName) == normalise(current) {
		return fmt.Sprintf("you are up to date (%s)", current), nil
	}

	asset, err := findAsset(release.Assets)
	if err != nil {
		return "", err
	}

	body, err := downloadFn(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", asset.Name, err)
	}
	defer body.Close()

	if err := applyFn(body); err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			return "", fmt.Errorf("update failed and rollback failed: %w", rerr)
		}
		return "", fmt.Errorf("applying update: %w", err)
	}

	return fmt.Sprintf("updated %s → %s", current, release.TagName), nil
}

// assetSuffix returns the expected release-asset suffix for the running OS/arch,
// matching the naming used by .github/workflows/release.yml (e.g. "darwin-arm64",
// "windows-amd64.exe").
func assetSuffix() string {
	suffix := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		suffix += ".exe"
	}
	return suffix
}

// findAsset picks the release asset matching the running OS/arch.
func findAsset(assets []ReleaseAsset) (ReleaseAsset, error) {
	suffix := assetSuffix()
	for _, a := range assets {
		if a.Name == "jira-thing-"+suffix {
			return a, nil
		}
	}
	return ReleaseAsset{}, fmt.Errorf("no release asset found for %s", suffix)
}

// downloadAsset fetches a release asset's raw bytes as a stream.
func downloadAsset(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GitHub returned %d", resp.StatusCode)
	}
	return resp.Body, nil
}
