package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// storyPointFields lists Jira field names that may hold story points (read-only check).
// Kept in sync by hand with the CLI's point-check command (main.go).
var storyPointFields = []string{"story_points", "customfield_10016", "customfield_10308"}

func registerPointCheckTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(checkStoryPointsTool(), handleCheckStoryPoints(conn))
}

func checkStoryPointsTool() mcp.Tool {
	return mcp.NewTool("check_story_points",
		mcp.WithDescription("Check the current user's open-sprint tickets for missing story points. Checks a few "+
			"common story-point field names/IDs; if this instance uses a different custom field, use list_fields "+
			"to find its ID and inspect tickets with describe_ticket instead."),
	)
}

func handleCheckStoryPoints(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fields := append([]string{"summary", "status", "priority", "updated"}, storyPointFields...)
		result, err := api.SearchIssues(conn, api.SearchQuery{
			JQL:        `assignee = currentUser() AND sprint in openSprints() ORDER BY updated DESC`,
			Fields:     fields,
			MaxResults: 100,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching sprint tickets: %v", err)), nil
		}
		if len(result.Issues) == 0 {
			return mcp.NewToolResultText("No tickets found in the current sprint."), nil
		}
		return mcp.NewToolResultText(formatMissingStoryPoints(result.Issues)), nil
	}
}

func formatMissingStoryPoints(issues []map[string]any) string {
	missing := missingStoryPointKeys(issues)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d ticket(s) checked, %d missing story points", len(issues), len(missing))
	if len(missing) == 0 {
		sb.WriteString(" — all good.\n")
		return sb.String()
	}
	sb.WriteString(":\n\n")
	for _, key := range missing {
		fmt.Fprintf(&sb, "- %s\n", key)
	}
	return sb.String()
}

func missingStoryPointKeys(issues []map[string]any) []string {
	var missing []string
	for _, issue := range issues {
		if !hasStoryPoints(issue) {
			key, _ := issue["key"].(string)
			missing = append(missing, key)
		}
	}
	return missing
}

func hasStoryPoints(issue map[string]any) bool {
	f, _ := issue["fields"].(map[string]any)
	if f == nil {
		return false
	}
	for _, field := range storyPointFields {
		if v, ok := f[field]; ok && v != nil {
			return true
		}
	}
	return false
}
