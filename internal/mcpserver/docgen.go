package mcpserver

import (
	"fmt"
	"sort"
	"strings"

	"jira-thing/internal/api"
)

// ToolsMarkdownTable renders every registered MCP tool as a "| Tool |
// Description |" markdown table, sorted by name. It builds a real server
// with a zero-value connection since tool registration only needs valid
// credentials at call time, not at schema-definition time.
func ToolsMarkdownTable() string {
	s := NewServer("dev", api.JiraConnection{})
	tools := s.ListTools()

	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("| Tool | Description |\n|---|---|\n")
	for _, name := range names {
		fmt.Fprintf(&sb, "| `%s` | %s |\n", name, tools[name].Tool.Description)
	}
	return sb.String()
}
