package main

import (
	"errors"
	"strings"

	"jira-thing/internal/profile"
)

// profileFlag is the global option naming the profile to use.
const profileFlag = "profile"

// errProfileValue means -profile was given without a name.
var errProfileValue = errors.New("-profile needs a name, e.g. jira-thing -profile work list")

// applyGlobalFlags handles options given before the command name: -profile
// NAME (also -profile=NAME and --profile). It selects the profile, which
// child processes inherit, and returns args with the options removed.
func applyGlobalFlags(args []string) ([]string, error) {
	if len(args) < 2 {
		return args, nil
	}
	rest := args[1:]
	for len(rest) > 0 {
		name, consumed, ok := profileArg(rest)
		if !ok {
			break
		}
		if name == "" {
			return nil, errProfileValue
		}
		if err := profile.Select(name); err != nil {
			return nil, err
		}
		rest = rest[consumed:]
	}
	return append([]string{args[0]}, rest...), nil
}

// profileArg reads a leading -profile option from args: the name, how many
// arguments it used, and whether args started with one at all.
func profileArg(args []string) (string, int, bool) {
	flagName, value, hasValue := strings.Cut(strings.TrimLeft(args[0], "-"), "=")
	if !strings.HasPrefix(args[0], "-") || flagName != profileFlag {
		return "", 0, false
	}
	if hasValue {
		return value, 1, true
	}
	if len(args) < 2 {
		return "", 1, true
	}
	return args[1], 2, true
}
