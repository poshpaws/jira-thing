package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerFieldTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(listFieldsTool(), handleListFields(conn))
}

func listFieldsTool() mcp.Tool {
	return mcp.NewTool("list_fields",
		mcp.WithDescription("List every field known to the Jira instance, including custom fields with their "+
			"customfield_XXXXX IDs. Custom fields are configured per instance, so their IDs must always be looked "+
			"up here rather than assumed — the same field name can map to a different ID on different boards."),
		mcp.WithString("search",
			mcp.Description("Optional case-insensitive substring to filter field names by (e.g. \"story point\")"),
		),
	)
}

func handleListFields(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fields, err := api.FetchFields(conn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching fields: %v", err)), nil
		}
		search := strings.ToLower(req.GetString("search", ""))
		return mcp.NewToolResultText(formatFields(fields, search)), nil
	}
}

func formatFields(fields []api.Field, search string) string {
	var sb strings.Builder
	matched := 0
	for _, f := range fields {
		if search != "" && !strings.Contains(strings.ToLower(f.Name), search) {
			continue
		}
		kind := "system"
		if f.Custom {
			kind = "custom"
		}
		fmt.Fprintf(&sb, "- %s (%s, %s)\n", f.Name, f.ID, kind)
		matched++
	}
	if matched == 0 {
		return "No matching fields found."
	}
	return sb.String()
}
