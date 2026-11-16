package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerWatchTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(watchTool("watch_ticket", "Start watching a Jira ticket, for yourself or someone else"), handleWatch(conn, true))
	s.AddTool(watchTool("unwatch_ticket", "Stop watching a Jira ticket, for yourself or someone else"), handleWatch(conn, false))
	s.AddTool(listWatchersTool(), handleListWatchers(conn))
}

// --- watch_ticket / unwatch_ticket ---

func watchTool(name, description string) mcp.Tool {
	return mcp.NewTool(name,
		mcp.WithDescription(description),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("user",
			mcp.Description(`Who: "me" (default), a name, an email or an account ID`),
		),
		dryRunOption("resolve the user and report the change"),
	)
}

func handleWatch(conn api.JiraConnection, watch bool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		wr := ops.WatchRequest{Key: key, Who: req.GetString("user", ""), Watch: watch}
		res, err := ops.WatchTicket(conn, wr, req.GetBool("dry_run", false))
		if err != nil {
			return mcp.NewToolResultError(assignErrorText(err)), nil
		}
		return mcp.NewToolResultText(formatWatch(res, watch)), nil
	}
}

// formatWatch reports the change, or what a dry run would do.
func formatWatch(res ops.WatchResult, watch bool) string {
	text := fmt.Sprintf("%s %s %s", ops.UserLabel(res.User), ops.WatchVerb(res, watch), res.Key)
	if res.Status == ops.StatusDryRun {
		return "Dry run: " + text
	}
	return text
}

// --- list_watchers ---

func listWatchersTool() mcp.Tool {
	return mcp.NewTool("list_watchers",
		mcp.WithDescription("List who is watching a Jira ticket"),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
	)
}

func handleListWatchers(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		users, err := ops.ListWatchers(conn, key)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(users) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("Nobody is watching %s.", termsafe.Line(key))), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Watching %s:\n", key)
		for _, u := range users {
			fmt.Fprintf(&sb, "- %s\n", ops.UserLabel(u))
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}
