package version

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	update "github.com/inconshreveable/go-update"
	"golang.org/x/mod/semver"
)

const (
	downloadTimeout  = 60 * time.Second
	maxAssetBytes    = 200 << 20 // 200 MiB, well above the largest built binary
	checksumsAsset   = "checksums.txt"
	checksumsSigName = "checksums.txt.sig"
)

// allowedDownloadHosts restricts asset downloads to GitHub's own release-asset hosts,
// even though the URL is sourced from the GitHub API rather than user input.
var allowedDownloadHosts = map[string]bool{
	"github.com":                            true,
	"objects.githubusercontent.com":         true,
	"github-releases.githubusercontent.com": true,
	"release-assets.githubusercontent.com":  true,
}

// downloadFn allows tests to stub out the asset download.
var downloadFn = downloadAsset

// applyFn allows tests to stub out the binary swap (it otherwise overwrites os.Executable()).
var applyFn = func(r io.Reader, checksum []byte) error {
	return update.Apply(r, update.Options{Checksum: checksum})
}

// downloadClient enforces a timeout and re-validates the host on every redirect hop.
var downloadClient = &http.Client{
	Timeout: downloadTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if err := validateDownloadURL(req.URL.String()); err != nil {
			return err
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
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
	if err := checkUpgrade(current, release.TagName); err != nil {
		if errors.Is(err, errAlreadyUpToDate) {
			return fmt.Sprintf("you are up to date (%s)", current), nil
		}
		return "", err
	}

	if err := downloadAndApply(ctx, release); err != nil {
		return "", err
	}
	return fmt.Sprintf("updated %s → %s", current, release.TagName), nil
}

// downloadAndApply picks the matching asset, verifies it against the signed checksums
// manifest, and swaps it in for the running binary.
func downloadAndApply(ctx context.Context, release *ReleaseInfo) error {
	asset, err := findAsset(release.Assets)
	if err != nil {
		return err
	}
	expectedSum, err := fetchExpectedChecksum(ctx, release.Assets, asset.Name)
	if err != nil {
		return err
	}

	body, err := downloadFn(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset.Name, err)
	}
	defer body.Close()

	if err := applyFn(body, expectedSum); err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed and rollback failed: %w", rerr)
		}
		return fmt.Errorf("applying update: %w", err)
	}
	return nil
}

// errAlreadyUpToDate is a sentinel used internally to short-circuit checkUpgrade.
var errAlreadyUpToDate = errors.New("already up to date")

// checkUpgrade refuses to install anything that isn't a strictly newer, validly
// formed semver release, preventing downgrade or malformed-tag installs.
func checkUpgrade(current, latestTag string) error {
	curV, latestV := "v"+normalise(current), "v"+normalise(latestTag)
	if !semver.IsValid(latestV) {
		return fmt.Errorf("latest release tag %q is not a valid version", latestTag)
	}
	if !semver.IsValid(curV) {
		return fmt.Errorf("current version %q is not a valid version", current)
	}
	switch semver.Compare(latestV, curV) {
	case 0:
		return errAlreadyUpToDate
	case -1:
		return fmt.Errorf("refusing to downgrade: latest release %s is older than current %s", latestTag, current)
	default:
		return nil
	}
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

// findAssetByName returns the release asset with the given exact name.
func findAssetByName(assets []ReleaseAsset, name string) (ReleaseAsset, error) {
	for _, a := range assets {
		if a.Name == name {
			return a, nil
		}
	}
	return ReleaseAsset{}, fmt.Errorf("release is missing %s", name)
}

// fetchExpectedChecksum downloads and verifies the signed checksums manifest published
// alongside the release, then returns the expected SHA256 for the named asset. This is
// the trust root for self-update: without a validly signed manifest, no binary is applied.
func fetchExpectedChecksum(ctx context.Context, assets []ReleaseAsset, assetName string) ([]byte, error) {
	manifestAsset, err := findAssetByName(assets, checksumsAsset)
	if err != nil {
		return nil, err
	}
	sigAsset, err := findAssetByName(assets, checksumsSigName)
	if err != nil {
		return nil, err
	}

	manifest, err := fetchBounded(ctx, manifestAsset.BrowserDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", checksumsAsset, err)
	}
	sig, err := fetchBounded(ctx, sigAsset.BrowserDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", checksumsSigName, err)
	}

	if err := verifyManifestSignature(manifest, sig); err != nil {
		return nil, fmt.Errorf("verifying release manifest signature: %w", err)
	}
	return parseChecksum(manifest, assetName)
}

// verifyManifestSignature checks the checksums manifest was signed by the release key.
func verifyManifestSignature(manifest, signature []byte) error {
	block, _ := pem.Decode([]byte(releaseSigningPubKey))
	if block == nil {
		return fmt.Errorf("embedded release signing public key is invalid")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parsing embedded public key: %w", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("embedded public key is not ECDSA")
	}

	sum := sha256.Sum256(manifest)
	return update.NewECDSAVerifier().VerifySignature(sum[:], signature, crypto.SHA256, ecPub)
}

// parseChecksum finds the "sha256sum  filename" line for assetName in a checksums.txt.
func parseChecksum(manifest []byte, assetName string) ([]byte, error) {
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != assetName {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil {
			return nil, fmt.Errorf("malformed checksum for %s: %w", assetName, err)
		}
		return sum, nil
	}
	return nil, fmt.Errorf("no checksum found for %s in %s", assetName, checksumsAsset)
}

// validateDownloadURL restricts downloads to GitHub's release-asset hosts over HTTPS.
func validateDownloadURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("refusing non-HTTPS download URL: %s", rawURL)
	}
	if !allowedDownloadHosts[u.Hostname()] {
		return fmt.Errorf("refusing download from untrusted host: %s", u.Hostname())
	}
	return nil
}

// fetchBounded downloads a small file (e.g. the checksums manifest or its signature)
// fully into memory, capped at maxAssetBytes.
func fetchBounded(ctx context.Context, rawURL string) ([]byte, error) {
	rc, err := downloadFn(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, maxAssetBytes)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// downloadAsset fetches a release asset's raw bytes as a stream, enforcing the host
// allowlist, a request timeout, and a maximum size.
func downloadAsset(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	if err := validateDownloadURL(rawURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GitHub returned %d", resp.StatusCode)
	}

	return &limitedReadCloser{r: io.LimitReader(resp.Body, maxAssetBytes), c: resp.Body}, nil
}

// limitedReadCloser pairs a size-limited Reader with the underlying Closer.
type limitedReadCloser struct {
	r io.Reader
	c io.Closer
}

func (l *limitedReadCloser) Read(p []byte) (int, error) { return l.r.Read(p) }
func (l *limitedReadCloser) Close() error               { return l.c.Close() }
