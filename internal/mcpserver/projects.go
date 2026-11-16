package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"

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
		projects, err := ops.ListProjects(conn)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(projects) == 0 {
			return mcp.NewToolResultText("No projects found."), nil
		}
		var sb strings.Builder
		for _, p := range projects {
			fmt.Fprintf(&sb, "- %s — %s\n", termsafe.Line(p.Key), termsafe.Line(p.Name))
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- list_versions ---

func listVersionsTool() mcp.Tool {
	return mcp.NewTool("list_versions",
		mcp.WithDescription("List a project's releases/versions, newest release date first. Archived releases are hidden unless include_archived is set."),
		mcp.WithString("project_key",
			mcp.Description("Jira project key (e.g. PROJ). Default: the configured project"),
		),
		mcp.WithBoolean("include_archived",
			mcp.Description("Also list archived releases"),
		),
		mcp.WithBoolean("unreleased_only",
			mcp.Description("Only list releases not yet released"),
		),
	)
}

func handleListVersions(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg, err := config.Load()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("loading config: %v", err)), nil
		}
		filter := ops.ReleaseFilter{
			Project:         ops.ProjectOrConfig(cfg, req.GetString("project_key", "")),
			IncludeArchived: req.GetBool("include_archived", false),
			UnreleasedOnly:  req.GetBool("unreleased_only", false),
		}
		versions, err := ops.ListReleases(conn, filter)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(formatReleases(filter.Project, versions)), nil
	}
}

// formatReleases lists releases as "- name (status, release date)".
func formatReleases(project string, versions []api.ProjectVersion) string {
	if len(versions) == 0 {
		return fmt.Sprintf("No releases found in %s.", project)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Releases in %s:\n", project)
	for _, v := range versions {
		fmt.Fprintf(&sb, "- %s (%s", termsafe.Line(v.Name), ops.ReleaseStatus(v))
		if v.ReleaseDate != "" {
			fmt.Fprintf(&sb, ", %s", termsafe.Line(v.ReleaseDate))
		}
		sb.WriteString(")\n")
	}
	return sb.String()
}
