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
	s.AddTool(unlinkTicketsTool(), handleUnlinkTickets(conn))
	s.AddTool(addRemoteLinkTool(), handleAddRemoteLink(conn))
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

// --- unlink_tickets ---

func unlinkTicketsTool() mcp.Tool {
	return mcp.NewTool("unlink_tickets",
		mcp.WithDescription("Remove the link between two Jira tickets, whichever link type currently connects them."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira key of one of the linked tickets"),
		),
		mcp.WithString("other_key",
			mcp.Required(),
			mcp.Description("Jira key of the other linked ticket"),
		),
	)
}

func handleUnlinkTickets(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		otherKey, err := req.RequireString("other_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := api.UnlinkIssues(conn, key, otherKey); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("unlinking %s from %s: %v", key, otherKey, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Unlinked %s from %s", key, otherKey)), nil
	}
}

// --- add_remote_link ---

func addRemoteLinkTool() mcp.Tool {
	return mcp.NewTool("add_remote_link",
		mcp.WithDescription("Attach a remote web link (e.g. a URL to external documentation or a related resource) to a Jira ticket."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to link"),
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Display text for the link"),
		),
	)
}

func handleAddRemoteLink(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		linkURL, err := req.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := api.AddRemoteLink(conn, key, linkURL, title); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("adding remote link to %s: %v", key, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Added remote link to %s", key)), nil
	}
}
