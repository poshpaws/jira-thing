package ops

import (
	"errors"
	"fmt"
	"maps"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
	"jira-thing/internal/template"
)

// DefaultIssueType is used when neither the base fields nor the request name one.
const DefaultIssueType = "Task"

// ErrSummaryRequired means a create had no summary.
var ErrSummaryRequired = errors.New("summary is required")

// CreateRequest describes a ticket to create. Fields is an optional base,
// such as a loaded template or a batch entry; every non-empty typed field
// overrides it. Bare-string project, issuetype and priority values are
// wrapped into Jira's object shape.
type CreateRequest struct {
	Fields      map[string]any
	Project     string
	Summary     string
	IssueType   string
	Priority    string
	Description string // markdown
	Labels      []string
	Components  []string
	Assignee    string // "me", "unassign" or an account ID
	Parent      string // epic/parent key, linked after the create
}

// CreateOptions controls how a create runs.
type CreateOptions struct {
	Dedupe *DedupeGuard // nil skips the duplicate check
	DryRun bool
	// Label is what a dry run records in Dedupe for later duplicates to
	// report, since no key exists yet. Defaults to "an earlier ticket in this run".
	Label string
}

// CreateResult reports what a create did. Payload is the POST body, set for
// both dry runs and real creates. ParentErr is set when the ticket was
// created but linking it to Parent failed.
type CreateResult struct {
	Key       string
	URL       string
	Status    string
	Payload   map[string]any
	Parent    string
	ParentErr error
}

// CreateTicket creates a Jira ticket from req, or reports the open duplicate
// that made it unnecessary, or (with DryRun) the payload it would send.
func CreateTicket(conn api.JiraConnection, req CreateRequest, opts CreateOptions) (CreateResult, error) {
	fields, err := createFields(conn, req)
	if err != nil {
		return CreateResult{}, err
	}
	existing, err := opts.Dedupe.Find(fields)
	if err != nil {
		return CreateResult{}, fmt.Errorf("%w; nothing created", err)
	}
	if existing != "" {
		return CreateResult{Key: existing, URL: IssueURL(conn, existing), Status: StatusExists}, nil
	}
	if opts.DryRun {
		return dryRunCreate(conn, fields, opts)
	}
	return submitCreate(conn, fields, opts.Dedupe)
}

// createFields overlays req's typed fields onto a copy of its base fields.
func createFields(conn api.JiraConnection, req CreateRequest) (map[string]any, error) {
	fields := maps.Clone(req.Fields)
	if fields == nil {
		fields = map[string]any{}
	}
	setIfGiven(fields, "project", req.Project)
	setIfGiven(fields, "summary", req.Summary)
	setIfGiven(fields, "issuetype", req.IssueType)
	setIfGiven(fields, "priority", req.Priority)
	setIfGiven(fields, "parent", req.Parent)
	if req.Description != "" {
		fields["description"] = req.Description
	}
	markdownDescription(fields)
	if len(req.Labels) > 0 {
		fields["labels"] = req.Labels
	}
	if len(req.Components) > 0 {
		fields["components"] = Components(req.Components)
	}
	if _, ok := fields["issuetype"]; !ok {
		fields["issuetype"] = DefaultIssueType
	}
	if str(fields, "summary") == "" {
		return nil, ErrSummaryRequired
	}
	return fields, setAssignee(conn, fields, req.Assignee)
}

// markdownDescription converts a string description (markdown, as written in
// a batch entry or passed by a caller) to ADF, dropping an empty one. An
// ADF description, as fetched into a template, is left alone.
func markdownDescription(fields map[string]any) {
	d, ok := fields["description"].(string)
	switch {
	case !ok:
	case d == "":
		delete(fields, "description")
	default:
		fields["description"] = adf.FromMarkdown(d)
	}
}

// dryRunCreate resolves the payload without sending it, recording the ticket
// in the guard so later duplicates in the same run are still caught.
func dryRunCreate(conn api.JiraConnection, fields map[string]any, opts CreateOptions) (CreateResult, error) {
	label := opts.Label
	if label == "" {
		label = "an earlier ticket in this run"
	}
	opts.Dedupe.Remember(fields, label)
	payload, parent, err := PreparePayload(conn, fields)
	if err != nil {
		return CreateResult{}, err
	}
	return CreateResult{Status: StatusDryRun, Payload: payload, Parent: parent}, nil
}

// submitCreate POSTs the ticket and links it to its parent, if it has one.
func submitCreate(conn api.JiraConnection, fields map[string]any, guard *DedupeGuard) (CreateResult, error) {
	payload, parent, err := PreparePayload(conn, fields)
	if err != nil {
		return CreateResult{}, err
	}
	created, err := api.CreateIssue(conn, payload)
	if err != nil {
		return CreateResult{}, fmt.Errorf("creating issue: %w", err)
	}
	key := str(created, "key")
	guard.Remember(fields, key)
	res := CreateResult{Key: key, URL: IssueURL(conn, key), Status: StatusCreated, Payload: payload, Parent: parent}
	if parent != "" {
		res.ParentErr = api.AddIssuesToEpic(conn, parent, []string{key})
	}
	return res, nil
}

// PreparePayload strips non-updatable fields, wraps bare reference fields,
// resolves a self-assignee marker, and extracts the optional "parent" key,
// returning the body CreateIssue sends and the parent key to link afterwards.
func PreparePayload(conn api.JiraConnection, fields map[string]any) (map[string]any, string, error) {
	payload := maps.Clone(fields)
	parent, _ := payload["parent"].(string)
	delete(payload, "parent")
	template.StripExcludedFields(payload)
	template.NormaliseRefFields(payload)
	if payload["assignee"] != template.AssigneeSelf {
		return payload, parent, nil
	}
	me, err := api.FetchMyself(conn)
	if err != nil {
		return nil, "", fmt.Errorf("fetching current user: %w", err)
	}
	template.ResolveAssignee(payload, str(me, "accountId"))
	return payload, parent, nil
}
