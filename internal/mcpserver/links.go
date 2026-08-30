package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerLinkTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(listLinkTypesTool(), handleListLinkTypes(conn))
	s.AddTool(linkTicketsTool(), handleLinkTickets(conn))
}

// --- list_link_types ---

func listLinkTypesTool() mcp.Tool {
	return mcp.NewTool("list_link_types",
		mcp.WithDescription("List the issue link types configured on this Jira instance (e.g. Blocks, Relates, "+
			"Duplicate). Link types are instance-configurable, so always call this before link_tickets rather than "+
			"assuming a type name is valid."),
	)
}

func handleListLinkTypes(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		types, err := api.FetchIssueLinkTypes(conn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching link types: %v", err)), nil
		}
		return mcp.NewToolResultText(formatLinkTypes(types)), nil
	}
}

func formatLinkTypes(types []api.IssueLinkType) string {
	var sb strings.Builder
	sb.WriteString("Available link types:\n\n")
	for _, t := range types {
		fmt.Fprintf(&sb, "- %s (outward: %q, inward: %q)\n", t.Name, t.Outward, t.Inward)
	}
	return sb.String()
}

// --- link_tickets ---

func linkTicketsTool() mcp.Tool {
	return mcp.NewTool("link_tickets",
		mcp.WithDescription("Link two Jira tickets together (e.g. mark one as blocking another). The link_type "+
			"must match one of the types returned by list_link_types — call that tool first if unsure."),
		mcp.WithString("outward_key",
			mcp.Required(),
			mcp.Description("Ticket the link's outward description applies to — e.g. for link_type \"Blocks\", "+
				"this is the ticket that blocks the other"),
		),
		mcp.WithString("inward_key",
			mcp.Required(),
			mcp.Description("Ticket the link's inward description applies to — e.g. for link_type \"Blocks\", "+
				"this is the ticket that is blocked"),
		),
		mcp.WithString("link_type",
			mcp.Required(),
			mcp.Description("Name of the link type (e.g. \"Blocks\", \"Relates\"), matched against list_link_types output"),
		),
	)
}

func handleLinkTickets(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		outwardKey, err := req.RequireString("outward_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		inwardKey, err := req.RequireString("inward_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		linkType, err := req.RequireString("link_type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := api.LinkIssues(conn, outwardKey, inwardKey, linkType); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("linking %s to %s: %v", outwardKey, inwardKey, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Linked %s -> %s (%s)", outwardKey, inwardKey, linkType)), nil
	}
}
