package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAgileTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(listBoardsTool(), handleListBoards(conn))
	s.AddTool(listSprintsTool(), handleListSprints(conn))
	s.AddTool(listSprintIssuesTool(), handleListSprintIssues(conn))
	s.AddTool(addToSprintTool(), handleAddToSprint(conn))
}

// --- list_boards ---

func listBoardsTool() mcp.Tool {
	return mcp.NewTool("list_boards",
		mcp.WithDescription("List Agile (Scrum/Kanban) boards visible to the current user, optionally filtered to a project."),
		mcp.WithString("project_key",
			mcp.Description("Optional Jira project key to filter boards to (e.g. PROJ)"),
		),
	)
}

func handleListBoards(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boards, err := api.FetchBoards(conn, req.GetString("project_key", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching boards: %v", err)), nil
		}
		if len(boards) == 0 {
			return mcp.NewToolResultText("No boards found."), nil
		}
		var sb strings.Builder
		for _, b := range boards {
			fmt.Fprintf(&sb, "- [%d] %s (%s)\n", b.ID, b.Name, b.Type)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- list_sprints ---

func listSprintsTool() mcp.Tool {
	return mcp.NewTool("list_sprints",
		mcp.WithDescription("List the sprints on a board. Get the board_id from list_boards."),
		mcp.WithNumber("board_id",
			mcp.Required(),
			mcp.Description("Board ID (from list_boards)"),
		),
		mcp.WithString("state",
			mcp.Description("Optional comma-separated filter: any of \"active\", \"future\", \"closed\""),
		),
	)
}

func handleListSprints(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID := int(req.GetFloat("board_id", 0))
		sprints, err := api.FetchSprints(conn, boardID, req.GetString("state", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching sprints: %v", err)), nil
		}
		if len(sprints) == 0 {
			return mcp.NewToolResultText("No sprints found."), nil
		}
		var sb strings.Builder
		for _, sp := range sprints {
			fmt.Fprintf(&sb, "- [%d] %s (%s)\n", sp.ID, sp.Name, sp.State)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- list_sprint_issues ---

func listSprintIssuesTool() mcp.Tool {
	return mcp.NewTool("list_sprint_issues",
		mcp.WithDescription("List the issues in a sprint. Get the sprint_id from list_sprints."),
		mcp.WithNumber("sprint_id",
			mcp.Required(),
			mcp.Description("Sprint ID (from list_sprints)"),
		),
		mcp.WithString("jql",
			mcp.Description("Optional JQL filter to narrow the issues returned (e.g. \"assignee = currentUser()\")"),
		),
	)
}

func handleListSprintIssues(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sprintID := int(req.GetFloat("sprint_id", 0))
		result, err := api.FetchSprintIssues(conn, sprintID, req.GetString("jql", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching sprint issues: %v", err)), nil
		}
		if len(result.Issues) == 0 {
			return mcp.NewToolResultText("No issues found in sprint."), nil
		}
		return mcp.NewToolResultText(formatSearchResults(result)), nil
	}
}

// --- add_to_sprint ---

func addToSprintTool() mcp.Tool {
	return mcp.NewTool("add_to_sprint",
		mcp.WithDescription("Move tickets into a sprint (up to 50 at a time)."),
		mcp.WithNumber("sprint_id",
			mcp.Required(),
			mcp.Description("Sprint ID (from list_sprints)"),
		),
		mcp.WithString("ticket_keys",
			mcp.Required(),
			mcp.Description("Comma-separated Jira ticket keys to add (e.g. \"PROJ-1,PROJ-2\")"),
		),
	)
}

func handleAddToSprint(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sprintID := int(req.GetFloat("sprint_id", 0))
		keysStr, err := req.RequireString("ticket_keys")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		keys := splitLabels(keysStr)
		if len(keys) == 0 {
			return mcp.NewToolResultError("no ticket keys provided"), nil
		}
		if err := api.AddIssuesToSprint(conn, sprintID, keys); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("adding to sprint: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Added %d ticket(s) to sprint %d", len(keys), sprintID)), nil
	}
}
