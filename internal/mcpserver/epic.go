package mcpserver

import (
	"context"
	"fmt"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerEpicTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(createEpicTool(), handleCreateEpic(conn))
	s.AddTool(listEpicIssuesTool(), handleListEpicIssues(conn))
	s.AddTool(addToEpicTool(), handleAddToEpic(conn))
	s.AddTool(removeFromEpicTool(), handleRemoveFromEpic(conn))
}

// --- create_epic ---

func createEpicTool() mcp.Tool {
	return mcp.NewTool("create_epic",
		mcp.WithDescription("Create an epic. Jira has two incompatible epic models depending on how the project is "+
			"configured (team-managed vs company-managed); this tool sets the classic \"Epic Name\" field when the "+
			"instance has one, and is a no-op on team-managed projects where it doesn't exist."),
		mcp.WithString("project",
			mcp.Required(),
			mcp.Description("Project key (e.g. PROJ)"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Epic name (shown on the board's epic panel; distinct from the summary)"),
		),
		mcp.WithString("summary",
			mcp.Required(),
			mcp.Description("Epic summary/title"),
		),
	)
}

func handleCreateEpic(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := req.RequireString("project")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		fields := api.BuildEpicFields(conn, project, name, summary)
		result, err := api.CreateIssue(conn, fields)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating epic: %v", err)), nil
		}
		key, _ := result["key"].(string)
		return mcp.NewToolResultText(fmt.Sprintf("Created epic %s", key)), nil
	}
}

// --- list_epic_issues ---

func listEpicIssuesTool() mcp.Tool {
	return mcp.NewTool("list_epic_issues",
		mcp.WithDescription("List the issues under an epic. Tries the \"parent\" field (team-managed projects) "+
			"then falls back to the classic \"Epic Link\" custom field (company-managed projects)."),
		mcp.WithString("epic_key",
			mcp.Required(),
			mcp.Description("Jira key of the epic (e.g. PROJ-1)"),
		),
	)
}

func handleListEpicIssues(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		epicKey, err := req.RequireString("epic_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, err := api.ListEpicIssues(conn, epicKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing issues for %s: %v", epicKey, err)), nil
		}
		if len(result.Issues) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No issues found under %s.", epicKey)), nil
		}
		return mcp.NewToolResultText(formatSearchResults(result)), nil
	}
}

// --- add_to_epic ---

func addToEpicTool() mcp.Tool {
	return mcp.NewTool("add_to_epic",
		mcp.WithDescription("Add issues to an epic. Tries the \"parent\" field then falls back to the classic "+
			"\"Epic Link\" custom field per issue."),
		mcp.WithString("epic_key",
			mcp.Required(),
			mcp.Description("Jira key of the epic (e.g. PROJ-1)"),
		),
		mcp.WithString("ticket_keys",
			mcp.Required(),
			mcp.Description("Comma-separated Jira ticket keys to add (e.g. \"PROJ-2,PROJ-3\")"),
		),
	)
}

func handleAddToEpic(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		epicKey, err := req.RequireString("epic_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		keysStr, err := req.RequireString("ticket_keys")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		keys := splitLabels(keysStr)
		if len(keys) == 0 {
			return mcp.NewToolResultError("no ticket keys provided"), nil
		}
		if err := api.AddIssuesToEpic(conn, epicKey, keys); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Added %d ticket(s) to %s", len(keys), epicKey)), nil
	}
}

// --- remove_from_epic ---

func removeFromEpicTool() mcp.Tool {
	return mcp.NewTool("remove_from_epic",
		mcp.WithDescription("Remove issues from their epic. Tries the \"parent\" field then falls back to the "+
			"classic \"Epic Link\" custom field per issue."),
		mcp.WithString("ticket_keys",
			mcp.Required(),
			mcp.Description("Comma-separated Jira ticket keys to remove from their epic (e.g. \"PROJ-2,PROJ-3\")"),
		),
	)
}

func handleRemoveFromEpic(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		keysStr, err := req.RequireString("ticket_keys")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		keys := splitLabels(keysStr)
		if len(keys) == 0 {
			return mcp.NewToolResultError("no ticket keys provided"), nil
		}
		if err := api.RemoveIssuesFromEpic(conn, keys); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Removed %d ticket(s) from their epic", len(keys))), nil
	}
}
