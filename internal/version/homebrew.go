package version

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrHomebrewManaged means the running binary belongs to Homebrew, which
// must do the upgrade: replacing it in place would desync brew's records.
var ErrHomebrewManaged = errors.New("jira-thing was installed with Homebrew; run `brew upgrade jira-thing` instead")

// checkNotHomebrew refuses to self-update a Homebrew-installed binary.
func checkNotHomebrew() error {
	path, err := executableFn()
	if err != nil {
		return nil // installBinary reports this with context later
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if homebrewManaged(path) {
		return ErrHomebrewManaged
	}
	return nil
}

// homebrewManaged reports whether path lies inside a Homebrew Cellar keg.
func homebrewManaged(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/Cellar/jira-thing/")
}
