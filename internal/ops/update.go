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
	Assignee    string // "me", "unassign", a name, an email or an account ID
	Parent      string // epic/parent key to link to
	IssueType   string // new issue type name, matched case-insensitively
	Custom      []CustomValue
	ListEdits   ListEdits // add/remove single labels, components and fix versions
	StoryPoints *float64
	// PointFields are the story point field IDs to try, first match on the
	// edit screen wins; nil means DefaultStoryPointFields.
	PointFields []string
}

// UpdateResult reports what an update did, or would do in a dry run.
type UpdateResult struct {
	Key         string
	URL         string
	Status      string
	Fields      map[string]any // fields set outright; empty when only Parent changes
	Update      map[string]any // add/remove verbs for list fields
	Parent      string
	IssueType   string // the new type's name, when the type changes
	PointsField string // the field story points were written to
}

// UpdateTicket applies req to an existing ticket: a field PUT when any fields
// were given, then a parent link when Parent was given. With dryRun it
// returns what it would send, without sending it.
func UpdateTicket(conn api.JiraConnection, req UpdateRequest, dryRun bool) (UpdateResult, error) {
	fields, err := updateFields(conn, req)
	if err != nil {
		return UpdateResult{}, err
	}
	update, err := updateVerbs(req.ListEdits, fields)
	if err != nil {
		return UpdateResult{}, err
	}
	if len(fields) == 0 && len(update) == 0 && req.Parent == "" && req.IssueType == "" && req.StoryPoints == nil {
		return UpdateResult{}, ErrNothingToUpdate
	}
	res := UpdateResult{Key: req.Key, URL: IssueURL(conn, req.Key), Status: StatusEdited, Fields: fields, Update: update, Parent: req.Parent}
	if err := applyMetaFields(conn, req, &res); err != nil {
		return UpdateResult{}, err
	}
	if dryRun {
		res.Status = StatusDryRun
		return res, nil
	}
	return res, submitUpdate(conn, req.Key, api.EditBody(fields, update), req.Parent)
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
	if err := mergeCustom(conn, fields, req.Custom); err != nil {
		return nil, err
	}
	return fields, setAssignee(conn, fields, req.Assignee)
}

// submitUpdate sends the edit PUT (if any) and the parent link (if any).
func submitUpdate(conn api.JiraConnection, key string, body map[string]any, parent string) error {
	if len(body) > 0 {
		if err := api.EditIssue(conn, key, body); err != nil {
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
