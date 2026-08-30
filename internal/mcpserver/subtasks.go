package mcpserver

import (
	"context"
	"fmt"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// subtaskInheritFields are the parent ticket fields copied onto each subtask,
// matching the CLI `subtask` command's behaviour.
var subtaskInheritFields = []string{"project", "priority", "labels", "components"}

func registerSubtaskTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(createSubtaskTool(), handleCreateSubtask(conn))
}

func createSubtaskTool() mcp.Tool {
	return mcp.NewTool("create_subtask",
		mcp.WithDescription("Create a subtask under an existing Jira ticket. Inherits project, priority, labels, "+
			"and components from the parent."),
		mcp.WithString("parent_key",
			mcp.Required(),
			mcp.Description("Jira key of the parent ticket (e.g. PROJ-42)"),
		),
		mcp.WithString("summary",
			mcp.Required(),
			mcp.Description("Subtask summary/title"),
		),
		mcp.WithString("description",
			mcp.Description("Subtask description in markdown format"),
		),
	)
}

func handleCreateSubtask(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parentKey, err := req.RequireString("parent_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		inherited, err := fetchInheritedFields(conn, parentKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching parent %s: %v", parentKey, err)), nil
		}

		fields := buildSubtaskFields(parentKey, inherited, summary, req.GetString("description", ""))
		result, err := api.CreateIssue(conn, fields)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating subtask: %v", err)), nil
		}
		key := ""
		if k, ok := result["key"].(string); ok {
			key = k
		}
		return mcp.NewToolResultText(fmt.Sprintf("Created subtask %s under %s", key, parentKey)), nil
	}
}

// fetchInheritedFields retrieves the parent ticket and extracts the fields to copy onto a subtask.
func fetchInheritedFields(conn api.JiraConnection, parentKey string) (map[string]any, error) {
	issue, err := api.FetchIssue(conn, parentKey)
	if err != nil {
		return nil, err
	}
	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parent ticket %s has no fields", parentKey)
	}
	inherited := make(map[string]any)
	for _, key := range subtaskInheritFields {
		if v, ok := fields[key]; ok && v != nil {
			inherited[key] = v
		}
	}
	return inherited, nil
}

// buildSubtaskFields assembles the fields map for creating a single subtask.
func buildSubtaskFields(parentKey string, inherited map[string]any, summary, description string) map[string]any {
	fields := make(map[string]any, len(inherited)+3)
	for k, v := range inherited {
		fields[k] = v
	}
	fields["summary"] = summary
	fields["issuetype"] = map[string]any{"name": "Sub-task"}
	fields["parent"] = map[string]any{"key": parentKey}
	if description != "" {
		fields["description"] = markdownToADFBody(description)
	}
	return fields
}
