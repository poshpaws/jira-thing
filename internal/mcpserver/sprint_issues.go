package mcpserver

import (
	"context"
	"fmt"
	"strconv"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// sprintOptions are the arguments that pick a sprint: a selector or an ID,
// plus the board to look the selector up on.
func sprintOptions(defaultHelp string) []mcp.ToolOption {
	return append([]mcp.ToolOption{
		mcp.WithString("sprint", mcp.Description(sprintHelp+defaultHelp)),
		mcp.WithNumber("sprint_id", mcp.Description("Sprint ID (from list_sprints); overrides sprint")),
	}, boardOptions()...)
}

// sprintSelector returns sprint_id when given, else sprint, else fallback.
func sprintSelector(req mcp.CallToolRequest, fallback string) string {
	if id := int(req.GetFloat("sprint_id", 0)); id > 0 {
		return strconv.Itoa(id)
	}
	return req.GetString("sprint", fallback)
}

// --- list_sprint_issues ---

func listSprintIssuesTool() mcp.Tool {
	opts := append([]mcp.ToolOption{
		mcp.WithDescription("List the issues in a sprint (up to 100). Defaults to the current sprint."),
		mcp.WithString("jql",
			mcp.Description("Optional JQL filter to narrow the issues returned (e.g. \"assignee = currentUser()\")"),
		),
	}, sprintOptions(`. Default: "current"`)...)
	return mcp.NewTool("list_sprint_issues", opts...)
}

func handleListSprintIssues(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		scope, err := sprintScope(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		got, err := ops.ListSprintIssues(conn, scope, sprintSelector(req, ops.SprintCurrent), req.GetString("jql", ""))
		if err != nil {
			return mcp.NewToolResultError(sprintErrorText(err)), nil
		}
		if len(got.Issues) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No issues found in sprint %d.", got.Sprint.ID)), nil
		}
		header := fmt.Sprintf("Sprint %s\n", sprintHeading(got.Sprint))
		return mcp.NewToolResultText(header + formatSearchResults(api.SearchResult{Issues: got.Issues})), nil
	}
}

// sprintHeading is the sprint's label, or just its ID when it was given by ID.
func sprintHeading(sp api.Sprint) string {
	if sp.Name == "" {
		return strconv.Itoa(sp.ID)
	}
	return sprintLabel(sp)
}

// --- add_to_sprint ---

func addToSprintTool() mcp.Tool {
	opts := append([]mcp.ToolOption{
		mcp.WithDescription("Move tickets into a sprint. More than 50 keys are sent in batches."),
		mcp.WithString("ticket_keys",
			mcp.Required(),
			mcp.Description("Comma-separated Jira ticket keys to add (e.g. \"PROJ-1,PROJ-2\")"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("If true, resolve the sprint and report what would move, without moving anything"),
		),
	}, sprintOptions(`. Required unless sprint_id is given`)...)
	return mcp.NewTool("add_to_sprint", opts...)
}

func handleAddToSprint(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		keysStr, err := req.RequireString("ticket_keys")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scope, err := sprintScope(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		addReq := ops.AddToSprintRequest{Scope: scope, Sprint: sprintSelector(req, ""), Keys: template.SplitLabels(keysStr)}
		res, err := ops.AddToSprint(conn, addReq, req.GetBool("dry_run", false))
		if err != nil {
			return mcp.NewToolResultError(sprintErrorText(err)), nil
		}
		return mcp.NewToolResultText(formatAddToSprint(res)), nil
	}
}

// formatAddToSprint reports the move, or what a dry run would do.
func formatAddToSprint(res ops.AddToSprintResult) string {
	verb := "Added"
	if res.Status == ops.StatusDryRun {
		verb = "Dry run: would add"
	}
	return fmt.Sprintf("%s %d ticket(s) to sprint %s", verb, len(res.Keys), sprintHeading(res.Sprint))
}
