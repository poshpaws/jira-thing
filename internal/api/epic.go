package api

import (
	"fmt"
	"strings"
)

// Jira has two incompatible epic models depending on how a project is configured:
//   - team-managed ("next-gen") projects link stories to an epic via the generic
//     "parent" field.
//   - company-managed ("classic") projects link stories via a custom "Epic Link"
//     field, whose ID (customfield_XXXXX) varies per instance.
// Every function here tries the parent-field model first and falls back to the
// Epic Link custom field, since which one applies isn't knowable without probing.

// findFieldByName looks up a field's ID by its exact display name (case-insensitive).
func findFieldByName(conn JiraConnection, name string) (string, error) {
	fields, err := FetchFields(conn)
	if err != nil {
		return "", err
	}
	for _, f := range fields {
		if strings.EqualFold(f.Name, name) {
			return f.ID, nil
		}
	}
	return "", fmt.Errorf("no field named %q on this instance", name)
}

// ListEpicIssues returns the issues under an epic, trying the "parent" field
// first and falling back to the classic "Epic Link" custom field.
func ListEpicIssues(conn JiraConnection, epicKey string) (SearchResult, error) {
	result, parentErr := SearchIssues(conn, SearchQuery{
		JQL:        fmt.Sprintf(`parent = %q`, epicKey),
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	})
	if parentErr == nil && result.Total > 0 {
		return result, nil
	}

	fieldID, err := findFieldByName(conn, "Epic Link")
	if err != nil {
		if parentErr != nil {
			return SearchResult{}, parentErr
		}
		return result, nil
	}
	return SearchIssues(conn, SearchQuery{
		JQL:        fmt.Sprintf(`%s = %q`, jqlCustomFieldRef(fieldID), epicKey),
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	})
}

// AddIssuesToEpic assigns issueKeys to epicKey, trying the "parent" field first
// and falling back to the classic "Epic Link" custom field per issue. Returns an
// error summarising any issues that failed on both attempts; issues that succeeded
// are still applied.
func AddIssuesToEpic(conn JiraConnection, epicKey string, issueKeys []string) error {
	epicLinkFieldID, _ := findFieldByName(conn, "Epic Link")

	var failed []string
	for _, key := range issueKeys {
		if err := UpdateIssue(conn, key, map[string]any{"parent": map[string]any{"key": epicKey}}); err == nil {
			continue
		}
		if epicLinkFieldID == "" {
			failed = append(failed, key)
			continue
		}
		if err := UpdateIssue(conn, key, map[string]any{epicLinkFieldID: epicKey}); err != nil {
			failed = append(failed, key)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to add to epic: %s", strings.Join(failed, ", "))
	}
	return nil
}

// RemoveIssuesFromEpic clears the epic link on each issue, trying the "parent"
// field first and falling back to the classic "Epic Link" custom field.
func RemoveIssuesFromEpic(conn JiraConnection, issueKeys []string) error {
	epicLinkFieldID, _ := findFieldByName(conn, "Epic Link")

	var failed []string
	for _, key := range issueKeys {
		if err := UpdateIssue(conn, key, map[string]any{"parent": nil}); err == nil {
			continue
		}
		if epicLinkFieldID == "" {
			failed = append(failed, key)
			continue
		}
		if err := UpdateIssue(conn, key, map[string]any{epicLinkFieldID: nil}); err != nil {
			failed = append(failed, key)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to remove from epic: %s", strings.Join(failed, ", "))
	}
	return nil
}

// BuildEpicFields assembles the fields payload for creating an epic. If the
// instance has an "Epic Name" custom field (classic/company-managed projects),
// it is populated with name; team-managed projects don't have this field, so it's
// simply omitted when not found.
func BuildEpicFields(conn JiraConnection, projectKey, name, summary string) map[string]any {
	fields := map[string]any{
		"project":   map[string]any{"key": projectKey},
		"summary":   summary,
		"issuetype": map[string]any{"name": "Epic"},
	}
	if epicNameFieldID, err := findFieldByName(conn, "Epic Name"); err == nil {
		fields[epicNameFieldID] = name
	}
	return fields
}

// jqlCustomFieldRef renders a custom field ID as a JQL field reference, e.g.
// "customfield_10014" -> "cf[10014]".
func jqlCustomFieldRef(fieldID string) string {
	numeric := strings.TrimPrefix(fieldID, "customfield_")
	return "cf[" + numeric + "]"
}
