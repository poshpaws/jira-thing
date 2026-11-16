package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// assigneeHelp describes the values every assignee argument accepts.
const assigneeHelp = `"me" for the current user, "unassign" to clear it, ` +
	"a name or email (looked up; it must match exactly one user, see search_users), or a Jira account ID"

func registerUserTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(searchUsersTool(), handleSearchUsers(conn))
	s.AddTool(assignTicketTool(), handleAssignTicket(conn))
}

// --- search_users ---

func searchUsersTool() mcp.Tool {
	return mcp.NewTool("search_users",
		mcp.WithDescription("Find active Jira users by name or email, returning their account IDs. "+
			"Use it to pick an assignee when a name matches several people."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Part of a display name or email address"),
		),
	)
}

func handleSearchUsers(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		users, err := ops.SearchUsers(conn, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(formatUsers(query, users)), nil
	}
}

// formatUsers lists users one per line, or says none matched.
func formatUsers(query string, users []ops.User) string {
	if len(users) == 0 {
		return fmt.Sprintf("No active users match %q.", query)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Users matching %q:\n\n", query)
	for _, u := range users {
		fmt.Fprintf(&sb, "- %s\n", ops.UserLabel(u))
	}
	return sb.String()
}

// --- assign_ticket ---

func assignTicketTool() mcp.Tool {
	return mcp.NewTool("assign_ticket",
		mcp.WithDescription("Assign a Jira ticket to someone, by name, email or account ID, or unassign it"),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("assignee",
			mcp.Required(),
			mcp.Description(assigneeHelp+`, or "default" for the project's default assignee`),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("If true, resolve the assignee and report it without changing the ticket"),
		),
	)
}

func handleAssignTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		assignee, err := req.RequireString("assignee")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		res, err := ops.AssignTicket(conn, ops.AssignRequest{Key: key, Assignee: assignee}, req.GetBool("dry_run", false))
		if err != nil {
			return mcp.NewToolResultError(assignErrorText(err)), nil
		}
		return mcp.NewToolResultText(formatAssignResult(res)), nil
	}
}

// assignErrorText adds a pointer to search_users when the name was ambiguous.
func assignErrorText(err error) string {
	var amb *ops.AmbiguousUserError
	if errors.As(err, &amb) {
		return err.Error() + "\nRetry with one of the account IDs above."
	}
	return err.Error()
}

// formatAssignResult reports the assignment, or what a dry run would do.
func formatAssignResult(res ops.AssignResult) string {
	who := ops.AssigneeLabel(res.Assignee)
	if res.Status == ops.StatusDryRun {
		return fmt.Sprintf("Dry run: would assign %s to %s\n%s", res.Key, who, res.URL)
	}
	return fmt.Sprintf("Assigned %s to %s\n%s", res.Key, who, res.URL)
}
