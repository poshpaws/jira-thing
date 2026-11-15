package mcpserver

import (
	"context"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLifecycleTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(whoamiTool(), handleWhoami(conn))
	s.AddTool(deleteTicketTool(), handleDeleteTicket(conn))
	s.AddTool(cloneTicketTool(), handleCloneTicket(conn))
	s.AddTool(addWorklogTool(), handleAddWorklog(conn))
}

// --- whoami ---

func whoamiTool() mcp.Tool {
	return mcp.NewTool("whoami",
		mcp.WithDescription("Show the current authenticated Jira user (account ID, display name, email)"),
	)
}

func handleWhoami(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		me, err := ops.WhoAmI(conn)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Account ID: %s\nDisplay name: %s\nEmail: %s",
			me.AccountID, me.DisplayName, me.Email)), nil
	}
}

// --- delete_ticket ---

func deleteTicketTool() mcp.Tool {
	return mcp.NewTool("delete_ticket",
		mcp.WithDescription("Permanently delete a Jira ticket. This cannot be undone — use with care."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithBoolean("cascade",
			mcp.Description("If true, also delete the ticket's subtasks. If false (default) and subtasks exist, the delete fails."),
		),
	)
}

func handleDeleteTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		cascade := req.GetBool("cascade", false)
		if err := ops.DeleteTicket(conn, key, cascade); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Deleted %s", key)), nil
	}
}

// --- clone_ticket ---

func cloneTicketTool() mcp.Tool {
	return mcp.NewTool("clone_ticket",
		mcp.WithDescription("Clone an existing Jira ticket, optionally overriding summary, priority, or assignee on the copy."),
		mcp.WithString("source_key",
			mcp.Required(),
			mcp.Description("Jira key of the ticket to clone"),
		),
		mcp.WithString("summary",
			mcp.Description("Override the cloned ticket's summary (default: same as source, prefixed \"CLONE - \")"),
		),
		mcp.WithString("priority",
			mcp.Description("Override the cloned ticket's priority name"),
		),
		mcp.WithString("assignee",
			mcp.Description("Assignee for the clone: \"me\", \"unassign\", or a Jira account ID"),
		),
	)
}

func handleCloneTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sourceKey, err := req.RequireString("source_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		res, err := ops.CloneTicket(conn, ops.CloneRequest{
			SourceKey: sourceKey,
			Summary:   req.GetString("summary", ""),
			Priority:  req.GetString("priority", ""),
			Assignee:  req.GetString("assignee", ""),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Cloned %s -> %s", res.SourceKey, res.Key)), nil
	}
}

// --- add_worklog ---

func addWorklogTool() mcp.Tool {
	return mcp.NewTool("add_worklog",
		mcp.WithDescription("Log time spent against a Jira ticket."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("time_spent",
			mcp.Required(),
			mcp.Description("Duration in Jira's format, e.g. \"2d 3h 30m\", \"1h\", \"45m\""),
		),
		mcp.WithString("comment",
			mcp.Description("Optional comment (markdown) describing the work done"),
		),
	)
}

func handleAddWorklog(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		timeSpent, err := req.RequireString("time_spent")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		err = ops.LogWork(conn, ops.WorklogRequest{Key: key, TimeSpent: timeSpent, Comment: req.GetString("comment", "")})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Logged %s on %s", timeSpent, key)), nil
	}
}
