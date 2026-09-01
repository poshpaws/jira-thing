// Command gendocs regenerates the MCP tools table in README.md from the
// tools actually registered on the MCP server, so the docs can't drift from
// the code. Run via `go generate ./...` or `make gen-docs`.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jira-thing/internal/mcpserver"
)

const (
	readmeName = "README.md"
	startMark  = "<!-- mcp-tools:start -->\n"
	endMark    = "\n<!-- mcp-tools:end -->"
)

// findReadme walks up from the working directory to locate README.md, so
// gendocs works whether invoked from the repo root (`make gen-docs`) or from
// this package's directory (`go generate`).
func findReadme() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, readmeName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found above %s", readmeName, dir)
		}
		dir = parent
	}
}

func main() {
	readmePath, err := findReadme()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		os.Exit(1)
	}

	original, err := os.ReadFile(readmePath) // #nosec G304 -- readmePath is found by walking cwd upward for README.md, not user input
	if err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: reading %s: %v\n", readmePath, err)
		os.Exit(1)
	}

	content := string(original)
	start := strings.Index(content, startMark)
	end := strings.Index(content, endMark)
	if start == -1 || end == -1 || end < start {
		fmt.Fprintf(os.Stderr, "gendocs: no mcp-tools markers found in %s\n", readmePath)
		os.Exit(1)
	}

	table := mcpserver.ToolsMarkdownTable()
	updated := content[:start+len(startMark)] + table + content[end:]

	if err := os.WriteFile(readmePath, []byte(updated), 0o600); err != nil { // #nosec G304,G703 -- readmePath is found by walking cwd upward for README.md, not user input
		fmt.Fprintf(os.Stderr, "gendocs: writing %s: %v\n", readmePath, err)
		os.Exit(1)
	}
}
