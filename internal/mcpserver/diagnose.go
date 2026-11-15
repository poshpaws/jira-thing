package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDiagnoseTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(diagnoseConnectionTool(), handleDiagnoseConnection(conn))
}

func diagnoseConnectionTool() mcp.Tool {
	return mcp.NewTool("diagnose_connection",
		mcp.WithDescription("Check jira-thing's Jira connection: the configured URL and email, the API token's format "+
			"(the token itself is masked), and an authenticated API call. Use when other tools fail with auth or "+
			"connection errors; the result suggests a fix for 401/403 failures."),
	)
}

func handleDiagnoseConnection(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(formatDiagnosis(ops.Diagnose(conn))), nil
	}
}

// formatDiagnosis lists each check and its outcome, stopping at the first failure.
func formatDiagnosis(d ops.Diagnosis) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "URL: %s\nEmail: %s\nToken: %s\n\n", d.URL, d.Email, d.MaskedToken)
	if d.TokenErr != nil {
		fmt.Fprintf(&sb, "FAIL token format: %v", d.TokenErr)
		return sb.String()
	}
	sb.WriteString("OK token format\n")
	if d.APIErr != nil {
		fmt.Fprintf(&sb, "FAIL API connection: %v", d.APIErr)
		if d.Hint != "" {
			fmt.Fprintf(&sb, "\nHint: %s", d.Hint)
		}
		return sb.String()
	}
	fmt.Fprintf(&sb, "OK API connection as %s (%s)\nAll checks passed.", d.User.DisplayName, d.User.AccountID)
	return sb.String()
}
