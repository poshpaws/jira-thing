package mcpserver

import (
	"context"
	"fmt"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAttachmentTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(addAttachmentTool(), handleAddAttachment(conn))
}

func addAttachmentTool() mcp.Tool {
	return mcp.NewTool("add_attachment",
		mcp.WithDescription("Upload a local file as an attachment on a Jira ticket. The file must be accessible on "+
			"the filesystem where jira-thing is running."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Absolute path to the local file to attach"),
		),
	)
}

func handleAddAttachment(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		filePath, err := req.RequireString("file_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result, err := api.AddAttachment(conn, key, filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("attaching %s to %s: %v", filePath, key, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Attached %d file(s) to %s", len(result), key)), nil
	}
}
