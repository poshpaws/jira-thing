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
// overrides it, and Defaults fill whatever both leave unset. Bare-string project, issuetype and priority values are
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
	Custom      []CustomValue
	FixVersions []string
	Sprint      string      // "current", "next" or a sprint ID, joined after the create
	SprintScope SprintScope // board Sprint is looked up on; zero means the project's only sprint board
	StoryPoints *float64    // set through the edit screen after the create
	PointFields []string    // story point field IDs to try; nil means DefaultStoryPointFields
	Defaults    CreateDefaults
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
// both dry runs and real creates. ParentErr, SprintErr and PointsErr are set
// when the ticket was created but that follow-up step failed.
type CreateResult struct {
	Key         string
	URL         string
	Status      string
	Payload     map[string]any
	Parent      string
	ParentErr   error
	Sprint      api.Sprint // zero ID: no sprint requested
	SprintErr   error
	StoryPoints *float64
	PointsField string // the field story points were written to
	PointsErr   error
}

// createPlan is everything a create sends: the POST fields, the custom
// field values merged into its payload, and the follow-ups after it.
type createPlan struct {
	fields, custom map[string]any
	followUps      followUps
}

// CreateTicket creates a Jira ticket from req, or reports the open duplicate
// that made it unnecessary, or (with DryRun) the payload it would send.
func CreateTicket(conn api.JiraConnection, req CreateRequest, opts CreateOptions) (CreateResult, error) {
	fields, err := createFields(conn, req)
	if err != nil {
		return CreateResult{}, err
	}
	// Custom values are kept apart from fields: PreparePayload strips every
	// customfield_* key, since templates carry stale ones.
	custom, err := ResolveCustomFields(conn, req.Custom)
	if err != nil {
		return CreateResult{}, err
	}
	follow, err := planFollowUps(conn, req, fields)
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
	plan := createPlan{fields: fields, custom: custom, followUps: follow}
	if opts.DryRun {
		return dryRunCreate(conn, plan, opts)
	}
	return submitCreate(conn, plan, opts.Dedupe)
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
	if len(req.FixVersions) > 0 {
		fields["fixVersions"] = NamedRefs(req.FixVersions)
	}
	req.Defaults.apply(fields)
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
func dryRunCreate(conn api.JiraConnection, plan createPlan, opts CreateOptions) (CreateResult, error) {
	label := opts.Label
	if label == "" {
		label = "an earlier ticket in this run"
	}
	opts.Dedupe.Remember(plan.fields, label)
	payload, parent, err := PreparePayload(conn, plan.fields)
	if err != nil {
		return CreateResult{}, err
	}
	maps.Copy(payload, plan.custom)
	res := CreateResult{Status: StatusDryRun, Payload: payload, Parent: parent}
	plan.followUps.report(&res)
	return res, nil
}

// submitCreate POSTs the ticket, links it to its parent if it has one, then
// runs the sprint and story point follow-ups.
func submitCreate(conn api.JiraConnection, plan createPlan, guard *DedupeGuard) (CreateResult, error) {
	payload, parent, err := PreparePayload(conn, plan.fields)
	if err != nil {
		return CreateResult{}, err
	}
	maps.Copy(payload, plan.custom)
	created, err := api.CreateIssue(conn, payload)
	if err != nil {
		return CreateResult{}, fmt.Errorf("creating issue: %w", err)
	}
	key := str(created, "key")
	guard.Remember(plan.fields, key)
	res := CreateResult{Key: key, URL: IssueURL(conn, key), Status: StatusCreated, Payload: payload, Parent: parent}
	if parent != "" {
		res.ParentErr = api.AddIssuesToEpic(conn, parent, []string{key})
	}
	plan.followUps.apply(conn, &res)
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
