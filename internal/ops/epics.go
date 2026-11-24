package ops

import (
	"fmt"

	"jira-thing/internal/api"
)

// ListEpics returns a project's epics, newest first.
func ListEpics(conn api.JiraConnection, project string) ([]map[string]any, error) {
	if project == "" {
		return nil, ErrProjectRequired
	}
	if err := api.ValidateProjectKey(project); err != nil {
		return nil, err
	}
	result, err := api.FetchEpics(conn, project)
	if err != nil {
		return nil, fmt.Errorf("listing epics in %s: %w", project, err)
	}
	return result.Issues, nil
}

// ListEpicIssues returns the issues under an epic, whichever epic model the
// project uses (parent field or Epic Link).
func ListEpicIssues(conn api.JiraConnection, epicKey string) ([]map[string]any, error) {
	result, err := api.ListEpicIssues(conn, epicKey)
	if err != nil {
		return nil, fmt.Errorf("listing issues in %s: %w", epicKey, err)
	}
	return result.Issues, nil
}
