package ops

import (
	"errors"
	"fmt"
	"strings"

	"jira-thing/internal/api"
)

// ErrHasSubtasks means a ticket with subtasks was deleted without cascade.
var ErrHasSubtasks = errors.New("ticket has subtasks; delete them too with cascade")

// DeleteRequest permanently deletes Key; Cascade also deletes its subtasks.
type DeleteRequest struct {
	Key     string
	Cascade bool
}

// DeleteResult describes the deleted (or, in a dry run, doomed) ticket.
type DeleteResult struct {
	Key      string
	Summary  string
	Subtasks []string
	Status   string
}

// InspectDelete returns what deleting a ticket would remove, without
// deleting it, so a front end can show it before asking to confirm.
func InspectDelete(conn api.JiraConnection, key string) (DeleteResult, error) {
	fields, err := fetchFields(conn, key)
	if err != nil {
		return DeleteResult{}, err
	}
	res := DeleteResult{Key: key, Summary: str(fields, "summary"), Subtasks: []string{}, Status: StatusDryRun}
	subtasks, _ := fields["subtasks"].([]any)
	for _, s := range subtasks {
		if m, ok := s.(map[string]any); ok {
			res.Subtasks = append(res.Subtasks, str(m, "key"))
		}
	}
	return res, nil
}

// DeleteTicket permanently deletes a ticket. It refuses one with subtasks
// unless Cascade is set. With dryRun it reports what would go, deleting nothing.
func DeleteTicket(conn api.JiraConnection, req DeleteRequest, dryRun bool) (DeleteResult, error) {
	res, err := InspectDelete(conn, req.Key)
	if err != nil {
		return DeleteResult{}, err
	}
	if len(res.Subtasks) > 0 && !req.Cascade {
		return DeleteResult{}, fmt.Errorf("%w: %s", ErrHasSubtasks, strings.Join(res.Subtasks, ", "))
	}
	if dryRun {
		return res, nil
	}
	if err := api.DeleteIssue(conn, req.Key, req.Cascade); err != nil {
		return DeleteResult{}, fmt.Errorf("deleting %s: %w", req.Key, err)
	}
	res.Status = StatusDeleted
	return res, nil
}
