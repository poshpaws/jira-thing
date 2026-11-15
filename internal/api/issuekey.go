package api

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrInvalidIssueKey means a value used as an issue key isn't one.
var ErrInvalidIssueKey = errors.New("invalid issue key")

// issueKeyPattern matches a Jira issue key (PROJ-42) or a numeric issue ID.
var issueKeyPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*-[0-9]+|[0-9]+)$`)

// ValidateIssueKey rejects anything that isn't an issue key or numeric ID.
// Keys are interpolated into request paths, so a crafted value such as
// "../../myself" or "X-1?fields=" must never reach the URL.
func ValidateIssueKey(key string) error {
	if !issueKeyPattern.MatchString(key) {
		return fmt.Errorf("%w %q: expected a key like PROJ-42", ErrInvalidIssueKey, key)
	}
	return nil
}
