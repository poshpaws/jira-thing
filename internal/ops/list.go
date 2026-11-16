package ops

import (
	"fmt"
	"slices"

	"jira-thing/internal/api"
)

const (
	// DefaultListLimit is how many tickets a list returns unless asked otherwise.
	DefaultListLimit = 100
	// MaxListLimit caps a list, to keep a stray query from paging all of Jira.
	MaxListLimit = 1000
)

// listBaseFields are always fetched, for the styled list and -json output.
var listBaseFields = []string{"summary", "status", "priority", "updated"}

// ListRequest runs Filter, returning up to Limit tickets (0 means
// DefaultListLimit) with the fields Columns need.
type ListRequest struct {
	Filter  ListFilter
	Columns []string
	Limit   int
}

// ListResult holds the matching issues and the JQL that found them.
type ListResult struct {
	JQL    string
	Issues []map[string]any
}

// ListTickets searches with a filter-built JQL query.
func ListTickets(conn api.JiraConnection, req ListRequest) (ListResult, error) {
	limit := req.Limit
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 1 || limit > MaxListLimit {
		return ListResult{}, fmt.Errorf("limit must be between 1 and %d", MaxListLimit)
	}
	jql, err := BuildListJQL(req.Filter)
	if err != nil {
		return ListResult{}, err
	}
	fields := slices.Clone(listBaseFields)
	for _, f := range columnFields(req.Columns) {
		if !slices.Contains(fields, f) {
			fields = append(fields, f)
		}
	}
	result, err := api.SearchAll(conn, api.SearchQuery{JQL: jql, Fields: fields}, limit)
	if err != nil {
		return ListResult{}, fmt.Errorf("searching: %w", err)
	}
	return ListResult{JQL: jql, Issues: result.Issues}, nil
}
