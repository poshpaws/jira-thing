package ops

import (
	"fmt"
	"strings"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
)

// StatusMoved reports a completed workflow transition.
const StatusMoved = "moved"

// TransitionError means the requested state isn't reachable from the
// ticket's current status. Its message lists the states that are.
type TransitionError struct {
	Key       string
	State     string
	Available []api.Transition
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("no transition to %q is available from the current state\n\n%s",
		e.State, FormatTransitions(e.Key, e.Available))
}

// TransitionRequest moves Key to State (matched case-insensitively), or
// straight through Transition when the caller already picked one from
// ListTransitions. Comment (markdown), Resolution and Assignee are applied
// as part of the same call, for workflows that require them.
type TransitionRequest struct {
	Key        string
	State      string
	Transition api.Transition
	Comment    string
	Resolution string
	Assignee   string // "me", "unassign" or an account ID
}

// TransitionResult reports the transition applied, or the payload a dry run would send.
type TransitionResult struct {
	Key        string
	URL        string
	Status     string
	Transition api.Transition
	Payload    map[string]any
}

// ListTransitions returns the workflow states key can currently move to.
func ListTransitions(conn api.JiraConnection, key string) ([]api.Transition, error) {
	transitions, err := api.FetchTransitions(conn, key)
	if err != nil {
		return nil, fmt.Errorf("fetching transitions for %s: %w", key, err)
	}
	return transitions, nil
}

// FindTransition returns the transition whose name matches state, ignoring case.
func FindTransition(transitions []api.Transition, state string) (api.Transition, bool) {
	for _, t := range transitions {
		if strings.EqualFold(t.Name, state) {
			return t, true
		}
	}
	return api.Transition{}, false
}

// FormatTransitions renders the available workflow states for key as a
// bullet list, the wording both the CLI and MCP show.
func FormatTransitions(key string, transitions []api.Transition) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Available states for %s:\n\n", key)
	for _, t := range transitions {
		fmt.Fprintf(&sb, "- %s\n", t.Name)
	}
	return sb.String()
}

// TransitionTicket moves a ticket to a new workflow state. With dryRun it
// returns the payload it would send, without sending it.
func TransitionTicket(conn api.JiraConnection, req TransitionRequest, dryRun bool) (TransitionResult, error) {
	transition, err := resolveTransition(conn, req)
	if err != nil {
		return TransitionResult{}, err
	}
	opts, err := transitionOptions(conn, req)
	if err != nil {
		return TransitionResult{}, err
	}
	res := TransitionResult{
		Key: req.Key, URL: IssueURL(conn, req.Key), Status: StatusMoved,
		Transition: transition, Payload: api.TransitionPayload(transition.ID, opts),
	}
	if dryRun {
		res.Status = StatusDryRun
		return res, nil
	}
	if err := api.TransitionIssueWithOptions(conn, req.Key, transition.ID, opts); err != nil {
		return TransitionResult{}, fmt.Errorf("transitioning %s: %w", req.Key, err)
	}
	return res, nil
}

// resolveTransition returns the caller's picked transition, or looks State up.
func resolveTransition(conn api.JiraConnection, req TransitionRequest) (api.Transition, error) {
	if req.Transition.ID != "" {
		return req.Transition, nil
	}
	transitions, err := ListTransitions(conn, req.Key)
	if err != nil {
		return api.Transition{}, err
	}
	transition, ok := FindTransition(transitions, req.State)
	if !ok {
		return api.Transition{}, &TransitionError{Key: req.Key, State: req.State, Available: transitions}
	}
	return transition, nil
}

// transitionOptions builds the transition's side effects from req.
func transitionOptions(conn api.JiraConnection, req TransitionRequest) (api.TransitionOptions, error) {
	fields := map[string]any{}
	if req.Resolution != "" {
		fields["resolution"] = map[string]any{"name": req.Resolution}
	}
	if err := setAssignee(conn, fields, req.Assignee); err != nil {
		return api.TransitionOptions{}, err
	}
	opts := api.TransitionOptions{Fields: fields}
	if req.Comment != "" {
		opts.Comment = adf.FromMarkdown(req.Comment)
	}
	return opts, nil
}
