package main

import (
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

// dedupeGuard finds tickets that would duplicate a template. Besides searching
// Jira it remembers what this run already handled, because a ticket created a
// moment ago may not be in Jira's search index yet. A nil guard (the flag is
// off) finds nothing.
type dedupeGuard struct {
	conn api.JiraConnection
	seen map[string]string
}

// newDedupeGuard returns a guard, or nil when the check isn't enabled.
func newDedupeGuard(conn api.JiraConnection, enabled bool) *dedupeGuard {
	if !enabled {
		return nil
	}
	return &dedupeGuard{conn: conn, seen: map[string]string{}}
}

// find returns the ticket tmpl would duplicate, or "" if there is none. An
// error means the check couldn't be completed, and the ticket must not be
// created: an unchecked create is exactly what the guard exists to prevent.
func (g *dedupeGuard) find(tmpl map[string]any) (string, error) {
	if g == nil {
		return "", nil
	}
	project, summary, err := dedupeScope(tmpl)
	if err != nil {
		return "", err
	}
	if existing, ok := g.seen[seenKey(project, summary)]; ok {
		return existing, nil
	}
	return api.FindOpenIssueBySummary(g.conn, project, summary)
}

// remember records label (a ticket key, or a description in a dry run) as
// what later templates with tmpl's project and summary duplicate.
func (g *dedupeGuard) remember(tmpl map[string]any, label string) {
	if g == nil {
		return
	}
	if project, summary, err := dedupeScope(tmpl); err == nil {
		g.seen[seenKey(project, summary)] = label
	}
}

// dedupeScope returns the project and summary a duplicate is matched on.
func dedupeScope(tmpl map[string]any) (project, summary string, err error) {
	template.NormaliseRefFields(tmpl)
	project = nestedString(tmpl, "project", "key")
	if project == "" {
		project = nestedString(tmpl, "project", "id")
	}
	if project == "" {
		return "", "", fmt.Errorf(`-dedupe-by-summary needs a project: set "project" in the template or pass -project`)
	}
	return project, getString(tmpl, "summary"), nil
}

// seenKey normalises a project and summary the way Jira matches are compared.
func seenKey(project, summary string) string {
	return project + "\x00" + strings.ToLower(strings.TrimSpace(summary))
}

// skipIfDuplicate reports whether flags ask for the duplicate check and it
// found a ticket, printing the notice if so. It exits if the check fails.
func skipIfDuplicate(conn api.JiraConnection, tmpl map[string]any, flags createFlags) bool {
	existing, err := newDedupeGuard(conn, flags.dedupe).find(tmpl)
	if err != nil {
		fatal("%v; nothing created", err)
	}
	if existing == "" {
		return false
	}
	reportDuplicate(conn, existing, flags.json)
	return true
}

// reportDuplicate prints the ticket that made a create unnecessary, as JSON
// when jsonOut is set, otherwise as styled text.
func reportDuplicate(conn api.JiraConnection, key string, jsonOut bool) {
	url := conn.BaseURL + "/browse/" + key
	if jsonOut {
		printJSONResult(jsonResult{Key: key, URL: url, Status: "exists"})
		return
	}
	fmt.Printf("%s %s already has this summary and is still open; nothing created\n",
		tui.SuccessStyle.Render("Skipped:"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s\n", url)
}
