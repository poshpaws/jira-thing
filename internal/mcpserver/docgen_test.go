package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestToolsMarkdownTable_MatchesREADME fails when a tool is added, removed,
// or redescribed without re-running `make gen-docs`, so README.md can't
// silently drift from the actual MCP tool set.
func TestToolsMarkdownTable_MatchesREADME(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}

	const startMark = "<!-- mcp-tools:start -->\n"
	const endMark = "\n<!-- mcp-tools:end -->"

	content := string(readme)
	start := strings.Index(content, startMark)
	end := strings.Index(content, endMark)
	if start == -1 || end == -1 || end < start {
		t.Fatal("README.md missing mcp-tools markers")
	}

	got := content[start+len(startMark) : end]
	want := ToolsMarkdownTable()

	if got != want {
		t.Errorf("README.md MCP tools table is out of date; run `make gen-docs`\n\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}
