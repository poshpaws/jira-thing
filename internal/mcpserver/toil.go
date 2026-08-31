package mcpserver

import (
	"context"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/config"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerToilTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(listToilTicketsTool(), handleListToilTickets(conn))
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
		if err := validateToilConfig(cfg); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		extra := ""
		if within := req.GetString("updated_within", ""); within != "" {
			extra = fmt.Sprintf("updated >= %s", within)
		}
		result, err := api.SearchIssues(conn, api.SearchQuery{
			JQL:        buildToilJQL(cfg, extra),
			Fields:     []string{"summary", "status", "priority", "updated"},
			MaxResults: 100,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("searching: %v", err)), nil
		}
		if len(result.Issues) == 0 {
			return mcp.NewToolResultText("No toil tickets found."), nil
		}
		return mcp.NewToolResultText(formatSearchResults(result)), nil
	}
}

// validateToilConfig checks that the config fields required for toil queries are set.
func validateToilConfig(cfg config.Config) error {
	if cfg.Project == "" || cfg.ToilMarker == "" {
		return fmt.Errorf("project and toil_marker must be set in ~/.config/jira-thing/jira-thing.json")
	}
	if cfg.UseTeamField && cfg.Team == "" {
		return fmt.Errorf("team must be set in config when use_team_field is true")
	}
	if !cfg.UseTeamField && cfg.ToilTeam == "" {
		return fmt.Errorf("toil_team must be set in config (or set use_team_field=true and team)")
	}
	return nil
}

// buildToilJQL constructs the JQL for toil ticket queries, ANDing in extra if provided.
func buildToilJQL(cfg config.Config, extra string) string {
	teamClause := fmt.Sprintf("labels = %q", cfg.ToilTeam)
	if cfg.UseTeamField {
		teamClause = fmt.Sprintf(`"Team[Team]" = %s`, cfg.Team)
	}
	jql := fmt.Sprintf("project = %q AND labels = %q AND %s", cfg.Project, cfg.ToilMarker, teamClause)
	if extra != "" {
		jql += " AND " + extra
	}
	return jql
}
