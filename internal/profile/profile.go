// Package profile names the active jira-thing profile: a separate set of
// stored credentials and config overrides, for working with more than one
// Jira site or account. No profile means the original, unnamed set.
package profile

import (
	"fmt"
	"os"
	"regexp"
)

// EnvVar selects the active profile; the -profile flag sets it too, so child
// processes such as serve-mcp inherit the choice.
const EnvVar = "JIRA_THING_PROFILE"

// namePattern keeps profile names safe to use in keyring keys.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// Validate rejects names that aren't letters, digits, "-" or "_" (1–32 long).
func Validate(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("profile name %q: use 1-32 letters, digits, '-' or '_'", name)
	}
	return nil
}

// Active returns the selected profile, "" for the default one.
func Active() (string, error) {
	name := os.Getenv(EnvVar)
	if name == "" {
		return "", nil
	}
	return name, Validate(name)
}

// Select makes name the active profile for this process and its children.
func Select(name string) error {
	if err := Validate(name); err != nil {
		return err
	}
	return os.Setenv(EnvVar, name)
}

// Key returns base for the default profile, or base@profile for a named one.
func Key(base, name string) string {
	if name == "" {
		return base
	}
	return base + "@" + name
}

// Label is the profile's name for messages: "default" when none is selected.
func Label(name string) string {
	if name == "" {
		return "default"
	}
	return name
}
