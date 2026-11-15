// Package ops holds jira-thing's feature logic. Each feature is implemented
// here once and called by the MCP tools, the CLI and the TUI alike, so agents
// and humans get identical behaviour. Functions take typed requests and return
// typed results; formatting the result is the caller's job.
package ops

import "jira-thing/internal/api"

// Result statuses shared by the front ends' text and JSON output.
const (
	StatusCreated = "created"
	StatusExists  = "exists"
	StatusEdited  = "edited"
	StatusDryRun  = "dry-run"
)

// IssueURL returns the browser URL for an issue key.
func IssueURL(conn api.JiraConnection, key string) string {
	return conn.BaseURL + "/browse/" + key
}

// str returns m[key] as a string, or "" if absent or not a string.
func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// nestedStr returns m[key1][key2] as a string, or "" if any level is missing.
func nestedStr(m map[string]any, key1, key2 string) string {
	inner, _ := m[key1].(map[string]any)
	return str(inner, key2)
}
