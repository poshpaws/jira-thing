package version

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// installFn allows tests to stub out the binary swap. The default implementation
// moves the already-downloaded-and-verified file at tempPath into place as the
// running executable, then removes tempPath.
var installFn = installBinary

// executableFn allows tests to stub out which path is treated as "the running
// executable" without needing to overwrite the actual test binary.
var executableFn = os.Executable

// downloadToTemp streams the asset to a file in the OS temp directory (honouring
// $TMPDIR), verifying its checksum before it is ever treated as installable. The
// caller owns the returned path and must remove it; on any error the temp file is
// cleaned up before returning.
func downloadToTemp(ctx context.Context, assetURL string, expectedSum []byte) (string, error) {
	body, err := downloadFn(ctx, assetURL)
	if err != nil {
		return "", err
	}
	defer body.Close()

	tempPath, sum, err := writeToTempFile(body)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(sum, expectedSum) {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("checksum mismatch: expected %x, got %x", expectedSum, sum)
	}
	if err := os.Chmod(tempPath, 0o755); err != nil { // #nosec G302 -- replacement must be executable
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("making %s executable: %w", tempPath, err)
	}
	return tempPath, nil
}

// writeToTempFile streams r into a new file under the OS temp directory, returning
// its path and SHA256 checksum. On error the caller need not clean up; only a
// successful return leaves a file on disk.
func writeToTempFile(r io.Reader) (path string, sum []byte, err error) {
	tmp, err := os.CreateTemp("", "jira-thing-update-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}
	tempPath := tmp.Name()

	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(tmp, hasher), r)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tempPath)
		return "", nil, fmt.Errorf("saving to %s: %w", tempPath, firstNonNil(copyErr, closeErr))
	}
	return tempPath, hasher.Sum(nil), nil
}

// firstNonNil returns the first non-nil error in errs, or nil if all are nil.
func firstNonNil(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// installBinary moves the already-downloaded, already-verified binary at tempPath
// into place as the running executable. The previous binary is renamed aside first
// so a failed final move can be rolled back, leaving the original binary intact.
func installBinary(tempPath string) error {
	defer func() { _ = os.Remove(tempPath) }() // no-op once successfully moved

	targetPath, err := executableFn()
	if err != nil {
		return fmt.Errorf("locating running executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(targetPath); err == nil {
		targetPath = resolved
	}

	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath) // clear any stale leftover from a previous failed update

	if err := os.Rename(targetPath, oldPath); err != nil {
		return explainInstallError(targetPath, err)
	}
	if err := moveFile(tempPath, targetPath); err != nil {
		if rerr := os.Rename(oldPath, targetPath); rerr != nil {
			return fmt.Errorf("update failed (%v), and restoring the previous binary also failed (%v) — %s may now be missing; restore it manually from %s", err, rerr, targetPath, oldPath)
		}
		return explainInstallError(targetPath, err)
	}
	_ = os.Remove(oldPath) // best-effort; a leftover .old is harmless and cleared next update
	return nil
}

// explainInstallError wraps a failure writing to targetPath with an actionable
// hint when the failure looks like a permissions problem — the common case being
// a user who doesn't own the binary's parent directory and so cannot replace it.
func explainInstallError(targetPath string, err error) error {
	if errors.Is(err, fs.ErrPermission) {
		dir := filepath.Dir(targetPath)
		return fmt.Errorf("permission denied writing to %s: %w (you likely don't own %s — install to a directory you own, or re-run with elevated privileges)", targetPath, err, dir)
	}
	return err
}

// moveFile renames src to dst, falling back to a copy when rename fails (e.g. src
// and dst are on different filesystems, such as a temp directory on another volume).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

// copyFile copies src to dst, replacing dst, and removes src on success.
func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 -- src is our own verified temp download
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) // #nosec G302,G304 -- replacement executable
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}
