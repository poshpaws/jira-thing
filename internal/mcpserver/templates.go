package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTemplateTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(fetchTemplateTool(), handleFetchTemplate(conn))
	s.AddTool(createTicketsBatchTool(), handleCreateTicketsBatch(conn))
}

// --- fetch_template ---

func fetchTemplateTool() mcp.Tool {
	return mcp.NewTool("fetch_template",
		mcp.WithDescription("Fetch a Jira ticket's reusable fields (project, issue type, priority, labels, components, "+
			"description, assignee) as a JSON template, for use as the base of create_tickets_batch entries. By default "+
			"the assignee becomes a marker meaning \"whoever creates from this template\". Saving a template as the CLI's "+
			"default is left to the user (jira-thing template KEY), so an agent can't change what later creates use."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key to copy (e.g. PROJ-42)"),
		),
		mcp.WithBoolean("keep_assignee",
			mcp.Description("If true, keep the ticket's original assignee instead of the creator marker"),
		),
	)
}

func handleFetchTemplate(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		tmpl, err := ops.FetchTemplate(conn, key, req.GetBool("keep_assignee", false))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching %s: %v", key, err)), nil
		}
		body, err := json.MarshalIndent(tmpl, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encoding template: %v", err)), nil
		}
		return mcp.NewToolResultText(string(body)), nil
	}
}

// --- create_tickets_batch ---

func createTicketsBatchTool() mcp.Tool {
	return mcp.NewTool("create_tickets_batch",
		mcp.WithDescription("Create several Jira tickets in one call. Each entry is a ticket object, e.g. "+
			`{"project":"PROJ","issuetype":"Task","summary":"...","description":"markdown","labels":["x"],"parent":"PROJ-10"}. `+
			"Entries are created independently: one failing does not stop the rest, and the result reports each one."),
		mcp.WithString("tickets",
			mcp.Required(),
			mcp.Description("JSON array of ticket objects. project and summary are required per entry; issuetype defaults "+
				"to Task; description is markdown; parent links the ticket to an epic after creation"),
		),
		mcp.WithBoolean("dedupe_by_summary",
			mcp.Description("If true, skip entries whose summary matches an open ticket in the same project, or an "+
				"earlier entry in this batch"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("If true, report what would be created without creating anything"),
		),
	)
}

func handleCreateTicketsBatch(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, err := req.RequireString("tickets")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var entries []map[string]any
		if err := json.Unmarshal([]byte(raw), &entries); err != nil || len(entries) == 0 {
			return mcp.NewToolResultError("tickets must be a non-empty JSON array of ticket objects"), nil
		}
		dryRun := req.GetBool("dry_run", false)
		items := ops.CreateTicketsBatch(conn, entries, req.GetBool("dedupe_by_summary", false), dryRun)
		return mcp.NewToolResultText(formatBatchItems(items, dryRun)), nil
	}
}

// formatBatchItems words each entry's outcome, then the tally.
func formatBatchItems(items []ops.BatchItem, dryRun bool) string {
	var sb strings.Builder
	for _, it := range items {
		fmt.Fprintf(&sb, "%d. %s\n", it.Index+1, formatBatchItem(it))
	}
	tally := ops.Tally(items)
	verb := "created"
	if dryRun {
		verb = "would be created"
	}
	fmt.Fprintf(&sb, "\n%d of %d %s, %d skipped as duplicates, %d failed\n",
		tally.Created, len(items), verb, tally.Skipped, tally.Failed)
	return sb.String()
}

// formatBatchItem words one entry's outcome.
func formatBatchItem(it ops.BatchItem) string {
	switch {
	case it.Err != nil:
		return fmt.Sprintf("Error (%s): %v", it.Summary, it.Err)
	case it.Result.Status == ops.StatusExists:
		return fmt.Sprintf("Skipped %q: duplicates %s", it.Summary, it.Result.Key)
	case it.Result.Status == ops.StatusDryRun:
		return fmt.Sprintf("Would create %q", it.Summary)
	default:
		return fmt.Sprintf("Created %s: %s (%s)", it.Result.Key, it.Summary, it.Result.URL)
	}
}
