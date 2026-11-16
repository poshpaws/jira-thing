package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/termsafe"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLifecycleTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(whoamiTool(), handleWhoami(conn))
	s.AddTool(deleteTicketTool(), handleDeleteTicket(conn))
	s.AddTool(cloneTicketTool(), handleCloneTicket(conn))
	s.AddTool(addWorklogTool(), handleAddWorklog(conn))
}

// dryRunOption is the dry_run argument shared by mutating tools; what says
// what the dry run reports.
func dryRunOption(what string) mcp.ToolOption {
	return mcp.WithBoolean("dry_run", mcp.Description("If true, "+what+" without changing anything"))
}

// prettyJSON renders v indented, for showing dry-run payloads.
func prettyJSON(v any) string {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("<failed to render payload: %v>", err)
	}
	return string(body)
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
			termsafe.Line(me.AccountID), termsafe.Line(me.DisplayName), termsafe.Line(me.Email))), nil
	}
}

// --- delete_ticket ---

func deleteTicketTool() mcp.Tool {
	return mcp.NewTool("delete_ticket",
		mcp.WithDescription("Permanently delete a Jira ticket. This cannot be undone: run with dry_run first and confirm with the user."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithBoolean("cascade",
			mcp.Description("If true, also delete the ticket's subtasks. If false (default) and subtasks exist, the delete is refused."),
		),
		dryRunOption("report the ticket and subtasks that would be deleted"),
	)
}

func handleDeleteTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		delReq := ops.DeleteRequest{Key: key, Cascade: req.GetBool("cascade", false)}
		res, err := ops.DeleteTicket(conn, delReq, req.GetBool("dry_run", false))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(formatDelete(res)), nil
	}
}

// formatDelete reports the deletion, or what a dry run would delete.
func formatDelete(res ops.DeleteResult) string {
	verb := "Deleted"
	if res.Status == ops.StatusDryRun {
		verb = "Dry run: would delete"
	}
	text := fmt.Sprintf("%s %s: %s", verb, termsafe.Line(res.Key), termsafe.Line(res.Summary))
	if len(res.Subtasks) > 0 {
		text += fmt.Sprintf("\nand its subtasks: %s", strings.Join(termsafe.Lines(res.Subtasks), ", "))
	}
	return text
}

// --- clone_ticket ---

func cloneTicketTool() mcp.Tool {
	return mcp.NewTool("clone_ticket",
		mcp.WithDescription("Clone an existing Jira ticket, optionally overriding summary, priority, assignee or labels, and replacing text in the summary and description."),
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
			mcp.Description("Assignee for the clone: "+assigneeHelp),
		),
		mcp.WithString("labels",
			mcp.Description("Comma-separated labels replacing the source's labels"),
		),
		mcp.WithArray("replace",
			mcp.Description(`Text replacements applied to the summary and description, each "find:replace" (split at the first colon)`),
			mcp.WithStringItems(),
		),
		dryRunOption("return the create payload"),
	)
}

func handleCloneTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cloneReq, err := cloneRequest(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		res, err := ops.CloneTicket(conn, cloneReq, req.GetBool("dry_run", false))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if res.Status == ops.StatusDryRun {
			return mcp.NewToolResultText(fmt.Sprintf("Dry run: would clone %s with\n%s", res.SourceKey, prettyJSON(res.Payload))), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Cloned %s -> %s\n%s", res.SourceKey, res.Key, res.URL)), nil
	}
}

// cloneRequest reads clone_ticket's arguments.
func cloneRequest(req mcp.CallToolRequest) (ops.CloneRequest, error) {
	sourceKey, err := req.RequireString("source_key")
	if err != nil {
		return ops.CloneRequest{}, err
	}
	cloneReq := ops.CloneRequest{
		SourceKey: sourceKey,
		Summary:   req.GetString("summary", ""),
		Priority:  req.GetString("priority", ""),
		Assignee:  req.GetString("assignee", ""),
	}
	if labels := req.GetString("labels", ""); labels != "" {
		cloneReq.Labels = template.SplitLabels(labels)
	}
	for _, pair := range req.GetStringSlice("replace", nil) {
		r, err := ops.ParseReplacement(pair)
		if err != nil {
			return ops.CloneRequest{}, err
		}
		cloneReq.Replace = append(cloneReq.Replace, r)
	}
	return cloneReq, nil
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
		dryRunOption("check the duration and report what would be logged"),
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
		wl := ops.WorklogRequest{Key: key, TimeSpent: timeSpent, Comment: req.GetString("comment", "")}
		res, err := ops.LogWork(conn, wl, req.GetBool("dry_run", false))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		verb := "Logged"
		if res.Status == ops.StatusDryRun {
			verb = "Dry run: would log"
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s %s on %s", verb, res.TimeSpent, res.Key)), nil
	}
}
