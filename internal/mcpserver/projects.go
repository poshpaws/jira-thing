package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerProjectTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(listProjectsTool(), handleListProjects(conn))
	s.AddTool(listVersionsTool(), handleListVersions(conn))
}

// --- list_projects ---

func listProjectsTool() mcp.Tool {
	return mcp.NewTool("list_projects",
		mcp.WithDescription("List every Jira project the current user can access"),
	)
}

func handleListProjects(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projects, err := api.FetchProjects(conn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching projects: %v", err)), nil
		}
		if len(projects) == 0 {
			return mcp.NewToolResultText("No projects found."), nil
		}
		var sb strings.Builder
		for _, p := range projects {
			fmt.Fprintf(&sb, "- %s — %s\n", p.Key, p.Name)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- list_versions ---

func listVersionsTool() mcp.Tool {
	return mcp.NewTool("list_versions",
		mcp.WithDescription("List the releases/versions configured on a project"),
		mcp.WithString("project_key",
			mcp.Required(),
			mcp.Description("Jira project key (e.g. PROJ)"),
		),
	)
}

func handleListVersions(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectKey, err := req.RequireString("project_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		versions, err := api.FetchProjectVersions(conn, projectKey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching versions for %s: %v", projectKey, err)), nil
		}
		if len(versions) == 0 {
			return mcp.NewToolResultText("No versions found."), nil
		}
		var sb strings.Builder
		for _, v := range versions {
			status := "unreleased"
			switch {
			case v.Archived:
				status = "archived"
			case v.Released:
				status = "released"
			}
			fmt.Fprintf(&sb, "- %s (%s)\n", v.Name, status)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}
