package ops

import (
	"errors"
	"fmt"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
)

// ErrNothingToUpdate means an update named no fields and no parent.
var ErrNothingToUpdate = errors.New("no fields provided to update")

// UpdateRequest names the fields to change on an existing ticket. Empty
// values are left untouched; Labels, when given, replaces all labels.
type UpdateRequest struct {
	Key         string
	Summary     string
	Description string // markdown, replaces the existing description
	Priority    string
	Labels      []string
	Assignee    string // "me", "unassign" or an account ID
	Parent      string // epic/parent key to link to
}

// UpdateResult reports what an update did, or would do in a dry run.
type UpdateResult struct {
	Key    string
	URL    string
	Status string
	Fields map[string]any // the PUT body; empty when only Parent changes
	Parent string
}

// UpdateTicket applies req to an existing ticket: a field PUT when any fields
// were given, then a parent link when Parent was given. With dryRun it
// returns what it would send, without sending it.
func UpdateTicket(conn api.JiraConnection, req UpdateRequest, dryRun bool) (UpdateResult, error) {
	fields, err := updateFields(conn, req)
	if err != nil {
		return UpdateResult{}, err
	}
	if len(fields) == 0 && req.Parent == "" {
		return UpdateResult{}, ErrNothingToUpdate
	}
	res := UpdateResult{Key: req.Key, URL: IssueURL(conn, req.Key), Status: StatusEdited, Fields: fields, Parent: req.Parent}
	if dryRun {
		res.Status = StatusDryRun
		return res, nil
	}
	return res, submitUpdate(conn, req.Key, fields, req.Parent)
}

// updateFields builds the PUT body from the fields req actually sets.
func updateFields(conn api.JiraConnection, req UpdateRequest) (map[string]any, error) {
	fields := map[string]any{}
	setIfGiven(fields, "summary", req.Summary)
	if req.Description != "" {
		fields["description"] = adf.FromMarkdown(req.Description)
	}
	if req.Priority != "" {
		fields["priority"] = map[string]any{"name": req.Priority}
	}
	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}
	return fields, setAssignee(conn, fields, req.Assignee)
}

// submitUpdate sends the field PUT (if any) and the parent link (if any).
func submitUpdate(conn api.JiraConnection, key string, fields map[string]any, parent string) error {
	if len(fields) > 0 {
		if err := api.UpdateIssue(conn, key, fields); err != nil {
			return fmt.Errorf("updating %s: %w", key, err)
		}
	}
	if parent == "" {
		return nil
	}
	if err := api.AddIssuesToEpic(conn, parent, []string{key}); err != nil {
		return fmt.Errorf("linking %s to parent %s: %w", key, parent, err)
	}
	return nil
}
