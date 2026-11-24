package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/config"
	"jira-thing/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// --- list_templates ---

func listTemplatesTool() mcp.Tool {
	return mcp.NewTool("list_templates",
		mcp.WithDescription("List the named ticket templates create_ticket can start from (its template argument): "+
			"the user's files in the templates directory, and the built-ins \"standard\" (a Task in the configured "+
			"project) and \"toil\" (adds the toil labels, when toil_marker is configured). A file overrides a built-in "+
			"of the same name. Templates are read-only to agents."),
	)
}

func handleListTemplates() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg, err := config.Load()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("loading config: %v", err)), nil
		}
		infos, err := ops.ListTemplates(cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var sb strings.Builder
		for _, info := range infos {
			fmt.Fprintf(&sb, "%s (%s)\n", info.Name, info.Source)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

// templateOption is create_ticket's template argument.
func templateOption() mcp.ToolOption {
	return mcp.WithString("template",
		mcp.Description("Name of a template (from list_templates) to start from; every other argument overrides its fields"),
	)
}

// templateFields loads the named template given as "template", or nil when
// none was given.
func templateFields(req mcp.CallToolRequest, cfg config.Config) (map[string]any, error) {
	name := req.GetString("template", "")
	if name == "" {
		return nil, nil
	}
	return ops.LoadTemplate(cfg, name)
}
