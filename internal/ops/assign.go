package ops

import (
	"errors"
	"fmt"

	"jira-thing/internal/api"
)

// AssigneeDefault hands a ticket to its project's default assignee. Only
// AssignTicket accepts it; field-level assignee arguments don't.
const AssigneeDefault = "default"

// StatusAssigned reports a completed assignment.
const StatusAssigned = "assigned"

// ErrAssigneeRequired means an assign request named nobody.
var ErrAssigneeRequired = errors.New(`assignee is required: "me", "unassign", "default", a name, an email or an account ID`)

// AssignRequest assigns Key to Assignee: "me", "unassign", "default", an
// account ID, or a name or email to look up.
type AssignRequest struct {
	Key      string
	Assignee string
	// Picked, when set, is used as-is instead of resolving Assignee: the
	// caller already chose it, e.g. from an AmbiguousUserError's candidates.
	Picked *User
}

// AssignResult reports the assignment made, or the body a dry run would send.
// Assignee is the zero User when the ticket was unassigned.
type AssignResult struct {
	Key      string
	URL      string
	Status   string
	Assignee User
	Payload  map[string]any
}

// AssignTicket sets a ticket's assignee. With dryRun it returns the body it
// would send, without sending it.
func AssignTicket(conn api.JiraConnection, req AssignRequest, dryRun bool) (AssignResult, error) {
	user, err := assignTarget(conn, req)
	if err != nil {
		return AssignResult{}, err
	}
	accountID := assignAccountID(user)
	res := AssignResult{
		Key: req.Key, URL: IssueURL(conn, req.Key), Status: StatusAssigned,
		Assignee: user, Payload: map[string]any{"accountId": accountID},
	}
	if dryRun {
		res.Status = StatusDryRun
		return res, nil
	}
	if err := api.AssignIssue(conn, req.Key, accountID); err != nil {
		return AssignResult{}, fmt.Errorf("assigning %s: %w", req.Key, err)
	}
	return res, nil
}

// AssigneeLabel describes who a ticket is assigned to, for both front ends.
func AssigneeLabel(u User) string {
	switch u.AccountID {
	case "":
		return "nobody (unassigned)"
	case api.DefaultAssigneeID:
		return "the project's default assignee"
	}
	if u.DisplayName == "" {
		return u.AccountID
	}
	return UserLabel(u)
}

// assignTarget resolves the request to a user; the zero User means unassign.
func assignTarget(conn api.JiraConnection, req AssignRequest) (User, error) {
	switch {
	case req.Picked != nil:
		return *req.Picked, nil
	case req.Assignee == "":
		return User{}, ErrAssigneeRequired
	case req.Assignee == AssigneeUnassign:
		return User{}, nil
	case req.Assignee == AssigneeDefault:
		return User{AccountID: api.DefaultAssigneeID}, nil
	}
	return ResolveUser(conn, req.Assignee)
}

// assignAccountID returns the account ID to send, or nil to unassign.
func assignAccountID(u User) *string {
	if u.AccountID == "" {
		return nil
	}
	return &u.AccountID
}
