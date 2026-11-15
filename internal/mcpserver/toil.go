package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerToilTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(listToilTicketsTool(), handleListToilTickets(conn))
	s.AddTool(syncToilTool(), handleSyncToil(conn))
}

func listToilTicketsTool() mcp.Tool {
	return mcp.NewTool("list_toil_tickets",
		mcp.WithDescription("List toil tickets for the project/team configured in "+
			"~/.config/jira-thing/jira-thing.json (project, toil_marker label, and team filter). "+
			"Fails if those aren't configured — this tool is specific to instances that have set up toil tracking."),
		mcp.WithString("updated_within",
			mcp.Description("Optional JQL relative-date filter on the updated field, e.g. \"-1w\", \"-30d\". Omit for no date filter."),
		),
	)
}

func handleListToilTickets(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg, err := config.Load()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("loading config: %v", err)), nil
		}
		extra := ""
		if within := req.GetString("updated_within", ""); within != "" {
			extra = fmt.Sprintf("updated >= %s", within)
		}
		issues, err := ops.ListToilTickets(conn, cfg, extra)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(issues) == 0 {
			return mcp.NewToolResultText("No toil tickets found."), nil
		}
		return mcp.NewToolResultText(formatSearchResults(api.SearchResult{Issues: issues})), nil
	}
}

// --- sync_toil ---

func syncToilTool() mcp.Tool {
	return mcp.NewTool("sync_toil",
		mcp.WithDescription("Sync open toil tickets to Confluence: one child page per ticket under the configured "+
			"ticket_hanger page (keeping any notes already written on it), then rewrite the hanger page's index. "+
			"Needs the toil settings plus confluence_space and ticket_hanger in ~/.config/jira-thing/jira-thing.json."),
		mcp.WithString("ticket_keys",
			mcp.Description("Comma-separated toil ticket keys to sync. Omit to sync every open toil ticket."),
		),
	)
}

func handleSyncToil(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg, err := config.Load()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("loading config: %v", err)), nil
		}
		issues, err := ops.OpenToilTickets(conn, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		selected := ops.FilterIssues(issues, template.SplitLabels(req.GetString("ticket_keys", "")))
		if len(selected) == 0 {
			return mcp.NewToolResultText("No matching open toil tickets; nothing synced."), nil
		}
		res, err := ops.SyncToilPages(conn, cfg, selected)
		text := formatToilSync(res)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("%s\nStopped: %v", text, err)), nil
		}
		return mcp.NewToolResultText(text), nil
	}
}

// formatToilSync lists each page synced and the hanger page updated.
func formatToilSync(res ops.ToilSyncResult) string {
	var sb strings.Builder
	for _, p := range res.Pages {
		fmt.Fprintf(&sb, "%s: %s\n", p.Action, p.Key)
	}
	fmt.Fprintf(&sb, "Synced %d ticket(s) to %q (%s)", len(res.Pages), res.Hanger, res.Space)
	return sb.String()
}
