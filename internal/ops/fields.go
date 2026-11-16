package ops

import "jira-thing/internal/api"

// Assignee argument values with special meaning; anything else is an account ID.
const (
	AssigneeMe       = "me"
	AssigneeUnassign = "unassign"
)

// ResolveAssignee turns an assignee argument into the Jira API's assignee
// field value: "unassign" clears the assignee (nil), and "me", an account ID,
// a name or an email resolves to that user's account ID (see ResolveUser).
func ResolveAssignee(conn api.JiraConnection, assignee string) (any, error) {
	if assignee == AssigneeUnassign {
		return nil, nil
	}
	user, err := ResolveUser(conn, assignee)
	if err != nil {
		return nil, err
	}
	return map[string]any{"accountId": user.AccountID}, nil
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
