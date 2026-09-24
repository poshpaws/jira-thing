package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// confluenceExportAttachDir is the subdirectory name (relative to output_dir)
// that downloaded attachments/embedded documents are written to, and that
// exported markdown image/link paths point at.
const confluenceExportAttachDir = "attachments"

func confluenceExportPageTool() mcp.Tool {
	return mcp.NewTool("confluence_export_page",
		mcp.WithDescription("Export a Confluence page as markdown, given its numeric page ID. Also resolves every embedded document the page references — images, attached files, view-file/multimedia macros, draw.io diagrams — downloading them into an attachments/ subdirectory when output_dir is given."),
		mcp.WithString("page_id",
			mcp.Required(),
			mcp.Description("Numeric Confluence page ID"),
		),
		mcp.WithString("output_dir",
			mcp.Description("Local directory to write index.md and an attachments/ subdirectory into. If omitted, only markdown text is returned and referenced attachments are listed but not downloaded."),
		),
	)
}

func handleConfluenceExportPage(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageID, err := req.RequireString("page_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		outputDir := req.GetString("output_dir", "")

		page, err := api.FetchConfluencePageBody(conn, pageID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching page: %v", err)), nil
		}
		markdown, attachments := markdownExporter(page.Body, confluenceExportAttachDir)

		if outputDir == "" {
			return mcp.NewToolResultText(formatConfluenceExportInline(page, markdown, attachments)), nil
		}
		return exportConfluencePageToDir(conn, page, markdown, attachments, outputDir)
	}
}

// exportConfluencePageToDir writes the exported markdown and downloads every
// referenced attachment under outputDir. A referenced filename with no
// matching attachment on the page (e.g. one since deleted) is reported as a
// warning in the result text rather than failing the whole export.
func exportConfluencePageToDir(conn api.JiraConnection, page api.ConfluencePageWithBody, markdown string, attachments []string, outputDir string) (*mcp.CallToolResult, error) {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("creating output directory: %v", err)), nil
	}
	var downloaded, missing []string
	if len(attachments) > 0 {
		attachDir := filepath.Join(outputDir, confluenceExportAttachDir)
		if err := os.MkdirAll(attachDir, 0o750); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating attachments directory: %v", err)), nil
		}
		var err error
		downloaded, missing, err = api.DownloadConfluenceAttachmentsByFilename(conn, page.ID, attachments, attachDir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("downloading attachments: %v", err)), nil
		}
	}
	mdPath := filepath.Join(outputDir, "index.md")
	content := fmt.Sprintf("# %s\n\n%s", page.Title, markdown)
	if err := os.WriteFile(mdPath, []byte(content), 0o644); err != nil { // #nosec G306 -- exported markdown is not sensitive
		return mcp.NewToolResultError(fmt.Sprintf("writing markdown file: %v", err)), nil
	}
	return mcp.NewToolResultText(formatConfluenceExportSaved(mdPath, downloaded, missing)), nil
}

// markdownExporter converts a Confluence storage-format XHTML body to
// markdown plus the attachment filenames it references. Set via
// SetMarkdownExporter from main — the converter lives in package main and
// can't be imported directly.
var markdownExporter func(storageXHTML, attachmentDir string) (markdown string, attachments []string)

// SetMarkdownExporter wires up the storage→markdown converter used by
// confluence_export_page. Panics if fn is nil — caller must wire before serving.
func SetMarkdownExporter(fn func(string, string) (string, []string)) {
	if fn == nil {
		panic("mcpserver: SetMarkdownExporter called with nil function")
	}
	markdownExporter = fn
}

// formatConfluenceExportInline formats an export result returned as text
// only (no output_dir given) — the markdown, plus a list of any referenced
// attachments the caller can fetch separately.
func formatConfluenceExportInline(page api.ConfluencePageWithBody, markdown string, attachments []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n%s", page.Title, markdown)
	if len(attachments) > 0 {
		sb.WriteString("\n---\nReferenced attachments (not downloaded — pass output_dir to save them):\n")
		for _, a := range attachments {
			fmt.Fprintf(&sb, "- %s\n", a)
		}
	}
	return sb.String()
}

// formatConfluenceExportSaved formats an export result after writing files to disk.
func formatConfluenceExportSaved(mdPath string, downloaded, missing []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Exported to %s\n", mdPath)
	if len(downloaded) > 0 {
		fmt.Fprintf(&sb, "%d attachment(s) downloaded: %s\n", len(downloaded), strings.Join(downloaded, ", "))
	}
	for _, m := range missing {
		fmt.Fprintf(&sb, "Warning: referenced attachment %q not found on page\n", m)
	}
	return sb.String()
}
