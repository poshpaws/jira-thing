package ops

import (
	"errors"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/template"
)

// ErrDedupeNeedsProject means a duplicate check was asked for on a ticket
// with no project to search in.
var ErrDedupeNeedsProject = errors.New(`duplicate check needs a project: set "project" in the template or pass a project`)

// DedupeGuard finds tickets that would duplicate a create. Besides searching
// Jira it remembers what this run already handled, because a ticket created a
// moment ago may not be in Jira's search index yet. A nil guard (the check is
// off) finds nothing.
type DedupeGuard struct {
	conn api.JiraConnection
	seen map[string]string
}

// NewDedupeGuard returns a guard, or nil when the check isn't enabled.
func NewDedupeGuard(conn api.JiraConnection, enabled bool) *DedupeGuard {
	if !enabled {
		return nil
	}
	return &DedupeGuard{conn: conn, seen: map[string]string{}}
}

// Find returns the ticket fields would duplicate, or "" if there is none. An
// error means the check couldn't be completed, and the ticket must not be
// created: an unchecked create is exactly what the guard exists to prevent.
func (g *DedupeGuard) Find(fields map[string]any) (string, error) {
	if g == nil {
		return "", nil
	}
	project, summary, err := dedupeScope(fields)
	if err != nil {
		return "", err
	}
	if existing, ok := g.seen[seenKey(project, summary)]; ok {
		return existing, nil
	}
	return api.FindOpenIssueBySummary(g.conn, project, summary)
}

// Remember records label (a ticket key, or a description in a dry run) as
// what later creates with the same project and summary duplicate.
func (g *DedupeGuard) Remember(fields map[string]any, label string) {
	if g == nil {
		return
	}
	if project, summary, err := dedupeScope(fields); err == nil {
		g.seen[seenKey(project, summary)] = label
	}
}

// dedupeScope returns the project and summary a duplicate is matched on.
func dedupeScope(fields map[string]any) (project, summary string, err error) {
	template.NormaliseRefFields(fields)
	project = nestedStr(fields, "project", "key")
	if project == "" {
		project = nestedStr(fields, "project", "id")
	}
	if project == "" {
		return "", "", ErrDedupeNeedsProject
	}
	return project, str(fields, "summary"), nil
}

// seenKey normalises a project and summary the way Jira matches are compared.
func seenKey(project, summary string) string {
	return project + "\x00" + strings.ToLower(strings.TrimSpace(summary))
}
