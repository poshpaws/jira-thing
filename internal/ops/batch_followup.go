package ops

import (
	"maps"
	"math"
	"strconv"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// Batch entry keys that aren't Jira fields: they become the create's sprint
// and story point follow-ups.
const (
	entrySprintKey = "sprint"
	entryPointsKey = "points"
)

// createBatchEntry creates one batch entry, reporting a failed follow-up
// (parent link, sprint, points) as its error.
func createBatchEntry(conn api.JiraConnection, entry map[string]any, batch BatchOptions, opts CreateOptions) (CreateResult, error) {
	req, err := batchEntryRequest(entry, batch)
	if err != nil {
		return CreateResult{}, err
	}
	res, err := CreateTicket(conn, req, opts)
	if err != nil {
		return res, err
	}
	return res, FollowUpError(res)
}

// batchEntryRequest builds an entry's create request, taking "sprint" and
// "points" out of its fields so they never reach the POST.
func batchEntryRequest(entry map[string]any, batch BatchOptions) (CreateRequest, error) {
	sprint, err := entrySprint(entry[entrySprintKey])
	if err != nil {
		return CreateRequest{}, err
	}
	points, err := entryPoints(entry[entryPointsKey])
	if err != nil {
		return CreateRequest{}, err
	}
	fields := maps.Clone(entry)
	delete(fields, entrySprintKey)
	delete(fields, entryPointsKey)
	req := CreateRequest{Fields: fields, Defaults: batch.Defaults, Sprint: sprint, StoryPoints: points, PointFields: batch.PointFields}
	if sprint != "" && batch.Scope != nil {
		req.SprintScope = batch.Scope(FieldsProject(fields))
	}
	return req, nil
}

// entrySprint reads an entry's sprint: a selector string, or a sprint ID
// written as a JSON number.
func entrySprint(v any) (string, error) {
	switch v := v.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case float64:
		if v > 0 && v == math.Trunc(v) {
			return strconv.Itoa(int(v)), nil
		}
	}
	return "", ErrBadSprint
}

// entryPoints reads an entry's story points, which must be a JSON number.
func entryPoints(v any) (*float64, error) {
	switch v := v.(type) {
	case nil:
		return nil, nil
	case float64:
		return &v, nil
	}
	return nil, ErrBadStoryPoints
}

// ConfigBatchOptions are the batch options that come from config: the
// defaults, the sprint board rule of CreateSprintScope and the story point
// fields. Callers add Dedupe and DryRun.
func ConfigBatchOptions(cfg config.Config) BatchOptions {
	return BatchOptions{
		Defaults:    DefaultsFromConfig(cfg),
		Scope:       func(project string) SprintScope { return CreateSprintScope(cfg, 0, project) },
		PointFields: StoryPointFields(cfg),
	}
}
