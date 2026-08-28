package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerConfluenceTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(confluenceBrowseTool(), handleConfluenceBrowse(conn))
	s.AddTool(confluenceGetPageTool(), handleConfluenceGetPage(conn))
	s.AddTool(confluenceCreatePageTool(), handleConfluenceCreatePage(conn))
	s.AddTool(confluenceUpdatePageTool(), handleConfluenceUpdatePage(conn))
}

// --- confluence_browse ---

func confluenceBrowseTool() mcp.Tool {
	return mcp.NewTool("confluence_browse",
		mcp.WithDescription("List child pages under a Confluence page. Use to navigate the page hierarchy."),
		mcp.WithString("page_id",
			mcp.Required(),
			mcp.Description("Numeric ID of the parent Confluence page"),
		),
	)
}

func handleConfluenceBrowse(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		parent, err := api.FetchConfluencePageByID(conn, pageID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching page: %v", err)), nil
		}
		children, err := api.ListChildPagesSummary(conn, pageID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing children: %v", err)), nil
		}
		return mcp.NewToolResultText(formatConfluenceChildren(parent, children)), nil
	}
}

// --- confluence_get_page ---

func confluenceGetPageTool() mcp.Tool {
	return mcp.NewTool("confluence_get_page",
		mcp.WithDescription("Fetch a Confluence page by space key and title, or by page ID. Returns the page's storage-format body."),
		mcp.WithString("page_id",
			mcp.Description("Numeric page ID (use this OR space+title)"),
		),
		mcp.WithString("space",
			mcp.Description("Confluence space key (e.g. ICSCET). Required with title."),
		),
		mcp.WithString("title",
			mcp.Description("Page title. Required with space."),
		),
	)
}

func handleConfluenceGetPage(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID := req.GetString("page_id", "")
		space := req.GetString("space", "")
		title := req.GetString("title", "")

		if pageID != "" {
			page, err := api.FetchConfluencePageByID(conn, pageID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("fetching page: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Page: %s (ID: %s, version: %d)", page.Title, page.ID, page.Version)), nil
		}

		if space == "" || title == "" {
			return mcp.NewToolResultError("provide either page_id, or both space and title"), nil
		}

		page, err := api.FetchConfluencePage(conn, space, title)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching page: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Page: %s (ID: %s, version: %d)", page.Title, page.ID, page.Version)), nil
	}
}

// --- confluence_create_page ---

func confluenceCreatePageTool() mcp.Tool {
	return mcp.NewTool("confluence_create_page",
		mcp.WithDescription("Create a new Confluence page under a parent page. Content is provided as markdown and converted to Confluence storage format."),
		mcp.WithString("space",
			mcp.Required(),
			mcp.Description("Confluence space key (e.g. ICSCET)"),
		),
		mcp.WithString("parent_id",
			mcp.Required(),
			mcp.Description("Numeric ID of the parent page"),
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Title for the new page"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Page content in markdown format"),
		),
	)
}

func handleConfluenceCreatePage(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		space, err := req.RequireString("space")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		parentID, err := req.RequireString("parent_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title, err := req.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		storage := markdownToStorageBody(content)

		// Check if page already exists — update instead of create.
		children, listErr := api.ListChildPagesSummary(conn, parentID)
		if listErr == nil {
			for _, child := range children {
				if child.Title == title {
					updateErr := api.UpdateConfluencePage(conn, child.ID, child.Version, title, storage)
					if updateErr != nil {
						return mcp.NewToolResultError(fmt.Sprintf("updating existing page: %v", updateErr)), nil
					}
					return mcp.NewToolResultText(fmt.Sprintf("Updated existing page: %s (ID: %s, version: %d → %d)", title, child.ID, child.Version, child.Version+1)), nil
				}
			}
		}

		page, err := api.CreateConfluencePage(conn, space, title, parentID, storage)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating page: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Created page: %s (ID: %s)", title, page.ID)), nil
	}
}

// --- confluence_update_page ---

func confluenceUpdatePageTool() mcp.Tool {
	return mcp.NewTool("confluence_update_page",
		mcp.WithDescription("Update an existing Confluence page's content. Content is provided as markdown and converted to Confluence storage format."),
		mcp.WithString("page_id",
			mcp.Required(),
			mcp.Description("Numeric ID of the page to update"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("New page content in markdown format"),
		),
	)
}

func handleConfluenceUpdatePage(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		page, err := api.FetchConfluencePageByID(conn, pageID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching page: %v", err)), nil
		}

		storage := markdownToStorageBody(content)
		if err := api.UpdateConfluencePage(conn, page.ID, page.Version, page.Title, storage); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating page: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Updated page: %s (ID: %s, version: %d → %d)", page.Title, page.ID, page.Version, page.Version+1)), nil
	}
}

// --- formatters ---

func formatConfluenceChildren(parent api.ConfluencePage, children []api.ConfluencePage) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Page: %s (ID: %s)\n", parent.Title, parent.ID)
	if len(children) == 0 {
		sb.WriteString("\nNo child pages.")
		return sb.String()
	}
	fmt.Fprintf(&sb, "\n%d child page(s):\n", len(children))
	for _, c := range children {
		fmt.Fprintf(&sb, "- %s (ID: %s)\n", c.Title, c.ID)
	}
	return sb.String()
}

// markdownToStorageBody converts markdown to Confluence storage format XHTML.
// Set via SetStorageConverter from main.
var markdownToStorageBody func(md string) string

// SetStorageConverter wires up the markdown→Confluence storage converter.
func SetStorageConverter(fn func(string) string) {
	markdownToStorageBody = fn
}
