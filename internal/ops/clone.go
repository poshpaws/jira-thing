package ops

import (
	"errors"
	"fmt"
	"strings"

	"jira-thing/internal/api"
)

// ClonePrefix is prepended to the source summary when a clone gets no summary of its own.
const ClonePrefix = "CLONE - "

// cloneInheritFields are copied from the source ticket before any overrides.
var cloneInheritFields = []string{"project", "issuetype", "priority", "labels", "components", "description"}

// ErrBadReplacement means a find/replace pair wasn't "find:replace" with a non-empty find.
var ErrBadReplacement = errors.New(`replacement must be "find:replace" with something to find`)

// Replacement swaps every Find for Replace in a clone's summary and description.
type Replacement struct {
	Find    string
	Replace string
}

// ParseReplacement reads "find:replace", splitting at the first colon.
func ParseReplacement(s string) (Replacement, error) {
	find, replace, ok := strings.Cut(s, ":")
	if !ok || find == "" {
		return Replacement{}, fmt.Errorf("%w: %q", ErrBadReplacement, s)
	}
	return Replacement{Find: find, Replace: replace}, nil
}

// CloneRequest copies SourceKey into a new ticket. Empty overrides keep the
// source's values; an empty Summary becomes ClonePrefix + the source summary.
// Labels, when non-nil, replace the source's labels.
type CloneRequest struct {
	SourceKey string
	Summary   string
	Priority  string
	Assignee  string // "me", "unassign", a name, an email or an account ID
	Labels    []string
	Replace   []Replacement
}

// CloneResult reports the new ticket, or the body a dry run would send.
type CloneResult struct {
	SourceKey string
	Key       string
	URL       string
	Status    string
	Payload   map[string]any
}

// CloneTicket creates a copy of a ticket, inheriting its project, type,
// priority, labels, components and description, then applying Replace.
func CloneTicket(conn api.JiraConnection, req CloneRequest, dryRun bool) (CloneResult, error) {
	source, err := fetchFields(conn, req.SourceKey)
	if err != nil {
		return CloneResult{}, err
	}
	inherited := inheritForClone(source, req.Replace)
	if req.Labels != nil {
		delete(inherited, "labels")
	}
	created, err := CreateTicket(conn, CreateRequest{
		Fields: inherited, Summary: cloneSummary(req, source), Priority: req.Priority,
		Assignee: req.Assignee, Labels: req.Labels,
	}, CreateOptions{DryRun: dryRun})
	if err != nil {
		return CloneResult{}, fmt.Errorf("cloning %s: %w", req.SourceKey, err)
	}
	status := StatusCloned
	if dryRun {
		status = StatusDryRun
	}
	return CloneResult{SourceKey: req.SourceKey, Key: created.Key, URL: created.URL, Status: status, Payload: created.Payload}, nil
}

// inheritForClone copies the inheritable fields, with replacements applied
// to the description's text.
func inheritForClone(source map[string]any, replace []Replacement) map[string]any {
	inherited := map[string]any{}
	for _, k := range cloneInheritFields {
		if v, ok := source[k]; ok && v != nil {
			inherited[k] = v
		}
	}
	if desc, ok := inherited["description"]; ok && len(replace) > 0 {
		inherited["description"] = replaceInADF(desc, replace)
	}
	return inherited
}

// cloneSummary is the requested summary, or the prefixed source summary,
// with replacements applied either way.
func cloneSummary(req CloneRequest, source map[string]any) string {
	summary := req.Summary
	if summary == "" {
		summary = ClonePrefix + str(source, "summary")
	}
	return replaceText(summary, req.Replace)
}

// replaceText applies each replacement in order.
func replaceText(s string, replace []Replacement) string {
	for _, r := range replace {
		s = strings.ReplaceAll(s, r.Find, r.Replace)
	}
	return s
}

// replaceInADF returns a copy of an ADF node with replacements applied to
// every text node, leaving the source ticket's document untouched.
func replaceInADF(node any, replace []Replacement) any {
	switch n := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, v := range n {
			out[k] = replaceInADF(v, replace)
		}
		if text, ok := n["text"].(string); ok && n["type"] == "text" {
			out["text"] = replaceText(text, replace)
		}
		return out
	case []any:
		out := make([]any, len(n))
		for i, v := range n {
			out[i] = replaceInADF(v, replace)
		}
		return out
	}
	return node
}
