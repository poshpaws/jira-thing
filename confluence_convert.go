package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// attachmentExtensions lists file extensions treated as local attachments when
// linked from markdown. Any link pointing at a local file with one of these
// extensions is uploaded as a Confluence attachment instead of rendered as an
// <a href> (which would be a dead link in Confluence).
var attachmentExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".svg": true, ".bmp": true, ".ico": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".csv": true, ".zip": true, ".gz": true,
	".tar": true, ".json": true, ".yaml": true, ".yml": true, ".xml": true,
	".txt": true, ".log": true, ".drawio": true,
}

// imageExtensions lists file extensions rendered inline via <ac:image>.
// Other attachment types use the view-file macro instead.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".svg": true, ".bmp": true, ".ico": true,
}

// confluenceResult holds the converted storage-format XHTML and the local file
// paths referenced in the markdown that need uploading as Confluence attachments.
type confluenceResult struct {
	Storage         string
	AttachmentPaths []string
}

// markdownToConfluence converts markdown text to Confluence storage-format XHTML.
// Images are converted to <ac:image> tags referencing attachment filenames.
// Local file links are converted to Confluence attachment link macros.
// The baseDir is used to resolve relative paths for the attachment list.
func markdownToConfluence(md string, baseDir string) confluenceResult {
	src := []byte(md)
	parser := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough),
	).Parser()
	root := parser.Parse(text.NewReader(src))

	var attachments []string
	var sb strings.Builder
	renderConfBlocks(&sb, root, src, baseDir, &attachments)

	return confluenceResult{Storage: sb.String(), AttachmentPaths: attachments}
}

// renderConfBlocks renders all block-level children of parent.
func renderConfBlocks(sb *strings.Builder, parent ast.Node, src []byte, baseDir string, attachments *[]string) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		renderConfBlock(sb, c, src, baseDir, attachments)
	}
}

// renderConfBlock renders a single block-level AST node to Confluence storage XHTML.
func renderConfBlock(sb *strings.Builder, n ast.Node, src []byte, baseDir string, attachments *[]string) {
	switch node := n.(type) {
	case *ast.Paragraph:
		renderConfParagraph(sb, node, src, baseDir, attachments)
	case *ast.TextBlock:
		sb.WriteString("<p>")
		renderConfInlines(sb, n, src, baseDir, attachments)
		sb.WriteString("</p>")
	case *ast.Heading:
		fmt.Fprintf(sb, "<h%d>", node.Level)
		renderConfInlines(sb, n, src, baseDir, attachments)
		fmt.Fprintf(sb, "</h%d>", node.Level)
	case *ast.List:
		renderConfList(sb, node, src, baseDir, attachments)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		renderConfCodeBlock(sb, n, src)
	case *ast.Blockquote:
		sb.WriteString("<blockquote>")
		renderConfBlocks(sb, n, src, baseDir, attachments)
		sb.WriteString("</blockquote>")
	case *ast.ThematicBreak:
		sb.WriteString("<hr/>")
	case *east.Table:
		renderConfTable(sb, node, src, baseDir, attachments)
	}
}

// renderConfParagraph renders a paragraph, but if it contains only a single image
// it emits the image without wrapping <p> tags for cleaner Confluence rendering.
func renderConfParagraph(sb *strings.Builder, n ast.Node, src []byte, baseDir string, attachments *[]string) {
	if isSoloImage(n) {
		renderConfInlines(sb, n, src, baseDir, attachments)
		return
	}
	sb.WriteString("<p>")
	renderConfInlines(sb, n, src, baseDir, attachments)
	sb.WriteString("</p>")
}

// isSoloImage returns true if the paragraph contains exactly one child and it is an image.
func isSoloImage(n ast.Node) bool {
	first := n.FirstChild()
	if first == nil {
		return false
	}
	_, isImage := first.(*ast.Image)
	return isImage && first.NextSibling() == nil
}

// renderConfList renders an ordered or unordered list.
func renderConfList(sb *strings.Builder, list *ast.List, src []byte, baseDir string, attachments *[]string) {
	tag := "ul"
	if list.IsOrdered() {
		tag = "ol"
	}
	fmt.Fprintf(sb, "<%s>", tag)
	for c := list.FirstChild(); c != nil; c = c.NextSibling() {
		sb.WriteString("<li>")
		renderConfBlocks(sb, c, src, baseDir, attachments)
		sb.WriteString("</li>")
	}
	fmt.Fprintf(sb, "</%s>", tag)
}

// renderConfCodeBlock renders a fenced or indented code block using the Confluence code macro.
func renderConfCodeBlock(sb *strings.Builder, n ast.Node, src []byte) {
	sb.WriteString(`<ac:structured-macro ac:name="code" ac:schema-version="1">`)
	if fenced, ok := n.(*ast.FencedCodeBlock); ok {
		if lang := fenced.Language(src); len(lang) > 0 {
			fmt.Fprintf(sb, `<ac:parameter ac:name="language">%s</ac:parameter>`, html.EscapeString(string(lang)))
		}
	}
	code := confRawLines(n, src)
	fmt.Fprintf(sb, `<ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body>`, escapeCDATA(code))
	sb.WriteString(`</ac:structured-macro>`)
}

// confRawLines joins the source line segments of a block node into one string.
func confRawLines(n ast.Node, src []byte) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		sb.Write(seg.Value(src))
	}
	return sb.String()
}

// renderConfTable renders a markdown table as a Confluence XHTML table.
func renderConfTable(sb *strings.Builder, table *east.Table, src []byte, baseDir string, attachments *[]string) {
	sb.WriteString("<table><tbody>")
	for r := table.FirstChild(); r != nil; r = r.NextSibling() {
		sb.WriteString("<tr>")
		cellTag := "td"
		if _, ok := r.(*east.TableHeader); ok {
			cellTag = "th"
		}
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			fmt.Fprintf(sb, "<%s>", cellTag)
			renderConfInlines(sb, c, src, baseDir, attachments)
			fmt.Fprintf(sb, "</%s>", cellTag)
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString("</tbody></table>")
}

// renderConfInlines renders all inline children of parent.
func renderConfInlines(sb *strings.Builder, parent ast.Node, src []byte, baseDir string, attachments *[]string) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		renderConfInline(sb, c, src, baseDir, attachments)
	}
}

// renderConfInline renders a single inline AST node to Confluence storage XHTML.
func renderConfInline(sb *strings.Builder, n ast.Node, src []byte, baseDir string, attachments *[]string) {
	switch node := n.(type) {
	case *ast.Text:
		renderConfText(sb, node, src)
	case *ast.String:
		sb.WriteString(html.EscapeString(string(node.Value)))
	case *ast.Emphasis:
		tag := "em"
		if node.Level == 2 {
			tag = "strong"
		}
		fmt.Fprintf(sb, "<%s>", tag)
		renderConfInlines(sb, n, src, baseDir, attachments)
		fmt.Fprintf(sb, "</%s>", tag)
	case *ast.CodeSpan:
		sb.WriteString("<code>")
		sb.WriteString(html.EscapeString(confRawInlineText(n, src)))
		sb.WriteString("</code>")
	case *east.Strikethrough:
		sb.WriteString("<s>")
		renderConfInlines(sb, n, src, baseDir, attachments)
		sb.WriteString("</s>")
	case *ast.Link:
		renderConfLink(sb, node, src, baseDir, attachments)
	case *ast.AutoLink:
		url := string(node.URL(src))
		fmt.Fprintf(sb, `<a href="%s">%s</a>`, html.EscapeString(url), html.EscapeString(url))
	case *ast.Image:
		renderConfImage(sb, node, src, baseDir, attachments)
	default:
		renderConfInlines(sb, n, src, baseDir, attachments)
	}
}

// renderConfText renders a text node, handling soft and hard line breaks.
func renderConfText(sb *strings.Builder, t *ast.Text, src []byte) {
	s := string(t.Segment.Value(src))
	if s != "" {
		sb.WriteString(html.EscapeString(s))
	}
	if t.HardLineBreak() {
		sb.WriteString("<br/>")
	} else if t.SoftLineBreak() {
		sb.WriteString(" ")
	}
}

// renderConfLink renders a markdown link. External URLs become normal <a> tags.
// Local file links with known attachment extensions become Confluence attachment
// link macros, and the resolved path is collected for upload.
func renderConfLink(sb *strings.Builder, link *ast.Link, src []byte, baseDir string, attachments *[]string) {
	dest := string(link.Destination)

	if isExternalURL(dest) || !isAttachmentFile(dest) {
		fmt.Fprintf(sb, `<a href="%s">`, html.EscapeString(dest))
		renderConfInlines(sb, link, src, baseDir, attachments)
		sb.WriteString("</a>")
		return
	}

	filename := filepath.Base(dest)
	fullPath := resolveLocalPath(dest, baseDir)
	if fullPath == "" {
		fmt.Fprintf(sb, `<a href="%s">`, html.EscapeString(dest))
		renderConfInlines(sb, link, src, baseDir, attachments)
		sb.WriteString("</a>")
		return
	}
	*attachments = append(*attachments, fullPath)

	// Confluence attachment link macro — renders as a clickable link to the uploaded file.
	sb.WriteString(`<ac:link>`)
	fmt.Fprintf(sb, `<ri:attachment ri:filename="%s"/>`, html.EscapeString(filename))
	sb.WriteString(`<ac:plain-text-link-body><![CDATA[`)
	var linkText strings.Builder
	renderConfInlinesPlain(&linkText, link, src)
	sb.WriteString(escapeCDATA(linkText.String()))
	sb.WriteString(`]]></ac:plain-text-link-body>`)
	sb.WriteString(`</ac:link>`)
}

// renderConfImage renders a markdown image as a Confluence attachment reference.
// External URLs become <ac:image> with <ri:url>.
// .drawio files (or images with a .drawio sibling) use the draw.io macro for
// native rendering in Confluence Cloud.
// Other local file references become <ac:image> with <ri:attachment>,
// and the resolved path is appended to attachments for later upload.
func renderConfImage(sb *strings.Builder, img *ast.Image, src []byte, baseDir string, attachments *[]string) {
	dest := string(img.Destination)
	alt := confImageAlt(img, src)

	if isExternalURL(dest) {
		fmt.Fprintf(sb, `<ac:image ac:alt="%s"><ri:url ri:value="%s"/></ac:image>`,
			html.EscapeString(alt), html.EscapeString(dest))
		return
	}

	fullPath := resolveLocalPath(dest, baseDir)
	if fullPath == "" {
		return
	}

	// If a .drawio source file exists alongside the image, prefer it — draw.io
	// renders natively in Confluence with full text and styling fidelity.
	if drawioPath := findDrawioSibling(fullPath); drawioPath != "" {
		drawioName := filepath.Base(drawioPath)
		diagramName := strings.TrimSuffix(drawioName, filepath.Ext(drawioName))
		*attachments = append(*attachments, drawioPath)
		emitDrawioMacro(sb, diagramName)
		return
	}

	// .drawio file referenced directly as an image
	if isDrawioFile(fullPath) {
		drawioName := filepath.Base(fullPath)
		diagramName := strings.TrimSuffix(drawioName, filepath.Ext(drawioName))
		*attachments = append(*attachments, fullPath)
		emitDrawioMacro(sb, diagramName)
		return
	}

	filename := filepath.Base(dest)
	*attachments = append(*attachments, fullPath)
	fmt.Fprintf(sb, `<ac:image ac:alt="%s"><ri:attachment ri:filename="%s"/></ac:image>`,
		html.EscapeString(alt), html.EscapeString(filename))
}

// confImageAlt extracts the alt text from an image node's inline children.
func confImageAlt(img *ast.Image, src []byte) string {
	var sb strings.Builder
	for c := img.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			sb.Write(t.Segment.Value(src))
		}
	}
	return sb.String()
}

// renderConfInlinesPlain renders inline children as plain text (no HTML markup).
// Used inside CDATA sections where HTML tags are not permitted.
func renderConfInlinesPlain(sb *strings.Builder, parent ast.Node, src []byte) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			sb.Write(t.Segment.Value(src))
		} else {
			renderConfInlinesPlain(sb, c, src)
		}
	}
}

// confRawInlineText concatenates the raw text of an inline node's children (for code spans).
func confRawInlineText(n ast.Node, src []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			sb.Write(t.Segment.Value(src))
		}
	}
	return sb.String()
}

// resolveLocalPath resolves a file path against baseDir and validates that the
// result does not escape the base directory. Both relative paths (../foo) and
// absolute paths (/etc/passwd) are checked. Returns the cleaned absolute path,
// or an empty string if the path is outside baseDir.
func resolveLocalPath(dest, baseDir string) string {
	resolved := filepath.Clean(filepath.Join(baseDir, dest))
	cleanBase := filepath.Clean(baseDir)
	if !strings.HasPrefix(resolved, cleanBase+string(filepath.Separator)) && resolved != cleanBase {
		fmt.Fprintf(os.Stderr, "  warning: skipping %q — path escapes base directory %q\n", dest, baseDir)
		return ""
	}
	return resolved
}

// isExternalURL returns true if the string starts with http:// or https://.
func isExternalURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// isAttachmentFile returns true if the path has a file extension in the known
// attachment list (images, documents, archives, etc.).
func isAttachmentFile(dest string) bool {
	ext := strings.ToLower(filepath.Ext(dest))
	return attachmentExtensions[ext]
}


// isDrawioFile returns true if the path has a .drawio extension.
func isDrawioFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".drawio")
}

// findDrawioSibling checks whether a .drawio file exists alongside an image file.
// For example, given "/docs/diagrams/flow.svg", it checks for "/docs/diagrams/flow.drawio".
// Returns the .drawio path if it exists, or empty string if not.
// Only checks for SVG and PNG files — those are the common exports from draw.io.
func findDrawioSibling(imagePath string) string {
	ext := strings.ToLower(filepath.Ext(imagePath))
	if ext != ".svg" && ext != ".png" {
		return ""
	}
	base := strings.TrimSuffix(imagePath, filepath.Ext(imagePath))
	drawioPath := base + ".drawio"
	if _, err := os.Stat(drawioPath); err == nil { // #nosec G703 -- imagePath is pre-validated by resolveLocalPath
		return drawioPath
	}
	return ""
}

// emitDrawioMacro writes the Confluence draw.io macro XHTML for an attached diagram.
// The diagramName is the attachment filename without the .drawio extension.
func emitDrawioMacro(sb *strings.Builder, diagramName string) {
	fmt.Fprintf(sb,
		`<ac:structured-macro ac:name="drawio" ac:schema-version="1">`+
			`<ac:parameter ac:name="diagramName">%s</ac:parameter>`+
			`</ac:structured-macro>`,
		html.EscapeString(diagramName))
}


// escapeCDATA escapes the CDATA end sequence ]]> to prevent premature
// CDATA termination and XHTML injection in Confluence storage format.
func escapeCDATA(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
}
