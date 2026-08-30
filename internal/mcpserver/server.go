package mcpserver

import (
	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates a configured MCP server with all jira-thing tools registered.
// The conn provides authenticated access to the Jira/Confluence API.
func NewServer(version string, conn api.JiraConnection) *server.MCPServer {
	s := server.NewMCPServer(
		"jira-thing",
		version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	registerTicketTools(s, conn)
	registerConfluenceTools(s, conn)
	registerFieldTools(s, conn)
	registerAttachmentTools(s, conn)
	registerSubtaskTools(s, conn)
	registerLinkTools(s, conn)
	registerLifecycleTools(s, conn)
	registerAgileTools(s, conn)
	registerProjectTools(s, conn)
	registerEpicTools(s, conn)

	return s
}
