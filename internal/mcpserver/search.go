package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	// searchDefaultLimit and searchMaxLimit bound search_tickets results.
	searchDefaultLimit = 20
	searchMaxLimit     = 100
	// filterHelp explains the shorthand the user and field filters accept.
	filterHelp = `. "x" means empty, "~x" not empty, "~value" not value`
)

// --- search_tickets ---

func searchTicketsTool() mcp.Tool {
	opts := []mcp.ToolOption{
		mcp.WithDescription("Search Jira tickets with JQL and/or filters (AND-ed together). Returns key, status, priority, summary and updated date, or the chosen columns."),
		mcp.WithString("jql", mcp.Description(`Raw JQL (e.g. 'project = PROJ AND status = "In Progress"'). With no other filter, its own ORDER BY is kept`)),
		mcp.WithNumber("max_results", mcp.Description(fmt.Sprintf("Maximum results (default %d, max %d)", searchDefaultLimit, searchMaxLimit))),
		mcp.WithString("columns", mcp.Description("Comma-separated columns to return as a table: "+strings.Join(ops.ColumnNames(), ","))),
	}
	for _, f := range searchStringFilters {
		opts = append(opts, mcp.WithString(f.arg, mcp.Description(f.help)))
	}
	opts = append(opts,
		mcp.WithBoolean("watching", mcp.Description("Only tickets you are watching")),
		mcp.WithBoolean("reverse", mcp.Description("Oldest first instead of newest first")),
	)
	return mcp.NewTool("search_tickets", opts...)
}

// searchStringFilter maps a string argument onto a ListFilter field.
type searchStringFilter struct {
	arg, help string
	set       func(f *ops.ListFilter, v string)
}

// searchStringFilters are search_tickets' string filter arguments.
var searchStringFilters = []searchStringFilter{
	{"assignee", `Assignee: "me", a name` + filterHelp, func(f *ops.ListFilter, v string) { f.Assignee = v }},
	{"reporter", `Reporter: "me", a name` + filterHelp, func(f *ops.ListFilter, v string) { f.Reporter = v }},
	{"status", "Status name" + filterHelp, func(f *ops.ListFilter, v string) { f.Status = v }},
	{"priority", "Priority name" + filterHelp, func(f *ops.ListFilter, v string) { f.Priority = v }},
	{"issue_type", "Issue type name" + filterHelp, func(f *ops.ListFilter, v string) { f.Type = v }},
	{"resolution", `Resolution name; "x" for unresolved`, func(f *ops.ListFilter, v string) { f.Resolution = v }},
	{"project_key", "Project key", func(f *ops.ListFilter, v string) { f.Project = v }},
	{"components", "Comma-separated components (any of)", func(f *ops.ListFilter, v string) { f.Components = template.SplitLabels(v) }},
	{"labels", "Comma-separated labels (any of)", func(f *ops.ListFilter, v string) { f.Labels = template.SplitLabels(v) }},
	{"created_after", `Created on or after: "-7d", "week", "2026-01-01"`, func(f *ops.ListFilter, v string) { f.Created = v }},
	{"created_before", `Created before: "-30d", "month", "2026-01-01"`, func(f *ops.ListFilter, v string) { f.CreatedBefore = v }},
	{"updated_after", `Updated on or after: "-1d", "week"`, func(f *ops.ListFilter, v string) { f.Updated = v }},
	{"order_by", `Sort field (default "created")`, func(f *ops.ListFilter, v string) { f.OrderBy = v }},
}

// searchFilter reads search_tickets' filter arguments.
func searchFilter(req mcp.CallToolRequest) ops.ListFilter {
	f := ops.ListFilter{JQL: req.GetString("jql", ""), Watching: req.GetBool("watching", false), Reverse: req.GetBool("reverse", false)}
	for _, sf := range searchStringFilters {
		if v := req.GetString(sf.arg, ""); v != "" {
			sf.set(&f, v)
		}
	}
	return f
}

func handleSearchTickets(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := int(req.GetFloat("max_results", searchDefaultLimit))
		if limit < 1 || limit > searchMaxLimit {
			limit = searchDefaultLimit
		}
		columns := req.GetString("columns", "")
		cols, err := ops.ParseColumns(columns)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		res, err := ops.ListTickets(conn, ops.ListRequest{Filter: searchFilter(req), Columns: cols, Limit: limit})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if columns == "" {
			return mcp.NewToolResultText(formatSearchResults(api.SearchResult{Issues: res.Issues})), nil
		}
		return mcp.NewToolResultText(formatColumns(conn, res.Issues, cols)), nil
	}
}

// formatColumns renders issues as a tab-separated table with a header row.
func formatColumns(conn api.JiraConnection, issues []map[string]any, cols []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d ticket(s):\n%s\n", len(issues), strings.Join(cols, "\t"))
	for _, issue := range issues {
		sb.WriteString(strings.Join(ops.TicketRow(conn, issue, cols), "\t") + "\n")
	}
	return sb.String()
}
