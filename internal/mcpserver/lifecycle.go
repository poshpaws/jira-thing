package mcpserver

import (
	"context"
	"fmt"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// cloneInheritFields are the fields copied from the source ticket when cloning,
// before any overrides supplied by the caller are applied.
var cloneInheritFields = []string{"project", "issuetype", "priority", "labels", "components", "description"}

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
		self, err := api.FetchMyself(conn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching current user: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Account ID: %v\nDisplay name: %v\nEmail: %v",
			self["accountId"], self["displayName"], self["emailAddress"])), nil
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
		if err := api.DeleteIssue(conn, key, cascade); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting %s: %v", key, err)), nil
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
		fields, err := buildCloneFields(conn, sourceKey, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, err := api.CreateIssue(conn, fields)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("cloning %s: %v", sourceKey, err)), nil
		}
		key, _ := result["key"].(string)
		return mcp.NewToolResultText(fmt.Sprintf("Cloned %s -> %s", sourceKey, key)), nil
	}
}

// buildCloneFields fetches the source ticket and assembles the fields for the clone,
// applying any caller-supplied overrides.
func buildCloneFields(conn api.JiraConnection, sourceKey string, req mcp.CallToolRequest) (map[string]any, error) {
	issue, err := api.FetchIssue(conn, sourceKey)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", sourceKey, err)
	}
	source, ok := issue["fields"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("ticket %s has no fields", sourceKey)
	}

	fields := make(map[string]any, len(cloneInheritFields)+3)
	for _, k := range cloneInheritFields {
		if v, ok := source[k]; ok && v != nil {
			fields[k] = v
		}
	}

	summary := req.GetString("summary", "")
	if summary == "" {
		if s, ok := source["summary"].(string); ok {
			summary = "CLONE - " + s
		}
	}
	fields["summary"] = summary

	if priority := req.GetString("priority", ""); priority != "" {
		fields["priority"] = map[string]any{"name": priority}
	}
	if assignee := req.GetString("assignee", ""); assignee != "" {
		resolved, err := resolveAssignee(conn, assignee)
		if err != nil {
			return nil, err
		}
		fields["assignee"] = resolved
	}
	return fields, nil
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
		var comment map[string]any
		if c := req.GetString("comment", ""); c != "" {
			comment = markdownToADFBody(c)
		}
		if err := api.AddWorklog(conn, key, timeSpent, comment); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("logging work on %s: %v", key, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Logged %s on %s", timeSpent, key)), nil
	}
}
