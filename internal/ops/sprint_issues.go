package ops

import (
	"fmt"

	"jira-thing/internal/api"
)

// SprintIssues is a sprint with the issues in it.
type SprintIssues struct {
	Sprint api.Sprint
	Issues []map[string]any
}

// ListSprintIssues returns the issues in the selected sprint ("current",
// "prev", "next" or an ID), optionally narrowed by a JQL filter.
func ListSprintIssues(conn api.JiraConnection, scope SprintScope, which, jql string) (SprintIssues, error) {
	sprint, err := FindSprint(conn, scope, which)
	if err != nil {
		return SprintIssues{}, err
	}
	result, err := api.FetchSprintIssues(conn, sprint.ID, jql)
	if err != nil {
		return SprintIssues{}, fmt.Errorf("fetching issues in sprint %d: %w", sprint.ID, err)
	}
	return SprintIssues{Sprint: sprint, Issues: result.Issues}, nil
}

// AddToSprintRequest moves Keys into the sprint named by Sprint ("current",
// "next" or an ID) on Scope's board.
type AddToSprintRequest struct {
	Scope  SprintScope
	Sprint string
	Keys   []string
}

// AddToSprintResult reports the issues moved, or the body a dry run would
// send (the first batch, when there are more than MaxSprintIssues).
type AddToSprintResult struct {
	Sprint  api.Sprint
	Keys    []string
	Status  string
	Payload map[string]any
}

// AddToSprint moves issues into a sprint, in batches of api.MaxSprintIssues.
// With dryRun it resolves the sprint and returns the body it would send.
func AddToSprint(conn api.JiraConnection, req AddToSprintRequest, dryRun bool) (AddToSprintResult, error) {
	if err := checkSprintKeys(req.Keys); err != nil {
		return AddToSprintResult{}, err
	}
	if req.Sprint == SprintPrev {
		return AddToSprintResult{}, ErrClosedSprintTarget
	}
	sprint, err := FindSprint(conn, req.Scope, req.Sprint)
	if err != nil {
		return AddToSprintResult{}, err
	}
	res := AddToSprintResult{
		Sprint: sprint, Keys: req.Keys, Status: StatusAdded,
		Payload: map[string]any{"issues": req.Keys[:min(len(req.Keys), api.MaxSprintIssues)]},
	}
	if dryRun {
		res.Status = StatusDryRun
		return res, nil
	}
	return res, sendToSprint(conn, sprint.ID, req.Keys)
}

// checkSprintKeys rejects an empty or malformed key list before any lookup.
func checkSprintKeys(keys []string) error {
	if len(keys) == 0 {
		return ErrNoIssueKeys
	}
	for _, key := range keys {
		if err := api.ValidateIssueKey(key); err != nil {
			return err
		}
	}
	return nil
}

// sendToSprint posts keys to a sprint in batches Jira accepts.
func sendToSprint(conn api.JiraConnection, sprintID int, keys []string) error {
	for start := 0; start < len(keys); start += api.MaxSprintIssues {
		batch := keys[start:min(start+api.MaxSprintIssues, len(keys))]
		if err := api.AddIssuesToSprint(conn, sprintID, batch); err != nil {
			return fmt.Errorf("adding to sprint %d: %w", sprintID, err)
		}
	}
	return nil
}
