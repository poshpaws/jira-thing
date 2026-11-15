package ops

import (
	"fmt"

	"jira-thing/internal/api"
)

// Assignee argument values with special meaning; anything else is an account ID.
const (
	AssigneeMe       = "me"
	AssigneeUnassign = "unassign"
)

// ResolveAssignee turns an assignee argument into the Jira API's assignee
// field value: "me" resolves to the current user's account ID, "unassign"
// clears the assignee (nil), and anything else is a literal account ID.
func ResolveAssignee(conn api.JiraConnection, assignee string) (any, error) {
	switch assignee {
	case AssigneeUnassign:
		return nil, nil
	case AssigneeMe:
		self, err := api.FetchMyself(conn)
		if err != nil {
			return nil, fmt.Errorf("resolving current user: %w", err)
		}
		return map[string]any{"accountId": self["accountId"]}, nil
	default:
		return map[string]any{"accountId": assignee}, nil
	}
}

// Components turns component names into the Jira API's component object shape.
func Components(names []string) []map[string]any {
	components := make([]map[string]any, len(names))
	for i, name := range names {
		components[i] = map[string]any{"name": name}
	}
	return components
}

// setIfGiven stores value under key unless value is empty.
func setIfGiven(fields map[string]any, key, value string) {
	if value != "" {
		fields[key] = value
	}
}

// setAssignee resolves assignee into fields, doing nothing when it is empty.
func setAssignee(conn api.JiraConnection, fields map[string]any, assignee string) error {
	if assignee == "" {
		return nil
	}
	resolved, err := ResolveAssignee(conn, assignee)
	if err != nil {
		return err
	}
	fields["assignee"] = resolved
	return nil
}
