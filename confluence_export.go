package main

import (
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// confluenceExport holds the markdown produced from a Confluence storage-format
// page body, plus the filenames of every attachment referenced within it —
// images, attachment links, and macro-embedded documents (view-file,
// multimedia, drawio, etc.) all resolve to attachment references, so the
// caller can download every one of them alongside the markdown.
type confluenceExport struct {
	Markdown    string
	Attachments []string // referenced attachment filenames, first-seen order, deduplicated
}

// confluenceExportState carries the shared output buffer and attachment
// dedupe set through the recursive render. The buffer is swapped out by
// renderSubtree to capture a nested block's markdown as a string (for table
// cells, list items, and blockquotes) without losing the accumulated
// attachment list.
type confluenceExportState struct {
	sb         strings.Builder
	attachment []string
	seenAttach map[string]bool
}

// renderCtx threads per-call rendering context (list nesting depth, the
// directory attachment links should point at) through the recursive walk.
type renderCtx struct {
	attachmentDir string
	listDepth     int
}

// confluenceStorageToMarkdown converts a Confluence storage-format XHTML page
// body to markdown. attachmentDir is the relative directory markdown image/
// link references point at (e.g. "attachments") — callers download the
// returned Attachments list into that directory alongside the markdown file.
func confluenceStorageToMarkdown(storageXHTML, attachmentDir string) confluenceExport {
	doc, err := html.Parse(strings.NewReader("<html><body>" + storageXHTML + "</body></html>"))
	if err != nil {
		// Malformed storage XHTML — fall back to the raw text so nothing is lost.
		return confluenceExport{Markdown: storageXHTML}
	}
	state := &confluenceExportState{seenAttach: map[string]bool{}}
	renderExportChildren(state, findDescendant(doc, "body"), renderCtx{attachmentDir: attachmentDir})
	md := collapseBlankLines(strings.TrimSpace(state.sb.String()))
	return confluenceExport{Markdown: md + "\n", Attachments: state.attachment}
}

// collapseBlankLines squashes runs of 3+ newlines down to a single blank line.
// Every block renderer emits its own trailing "\n\n", so adjacent blocks
// otherwise accumulate extra blank lines.
func collapseBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// renderExportChildren renders every child of parent in document order.
// Reused for both block contexts (page body, list items, blockquotes) and
// inline contexts (inside <strong>, <a>, ...) — block handlers are
// responsible for their own spacing, so plain iteration works everywhere.
func renderExportChildren(state *confluenceExportState, parent *html.Node, ctx renderCtx) {
	if parent == nil {
		return
	}
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		renderExportNode(state, c, ctx)
	}
}

// renderExportNode dispatches a single node: text is written verbatim
// (already entity-decoded by the HTML parser), elements are looked up in
// exportElementRenderers, and anything else (comments, doctypes) is dropped.
func renderExportNode(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	switch n.Type {
	case html.TextNode:
		state.sb.WriteString(n.Data)
	case html.ElementNode:
		if fn, ok := exportElementRenderers[n.Data]; ok {
			fn(state, n, ctx)
			return
		}
		// Unrecognised wrapper (div, span, ac:rich-text-body, ac:layout*, ...) —
		// pass through to its children rather than dropping the content.
		renderExportChildren(state, n, ctx)
	}
}

// renderSubtree renders n's children into a fresh buffer and returns the
// trimmed result, leaving state's attachment list intact. Used wherever a
// block's markdown needs capturing as a standalone string: table cells,
// list items, and blockquote/panel bodies.
func renderSubtree(state *confluenceExportState, n *html.Node, ctx renderCtx) string {
	saved := state.sb
	state.sb = strings.Builder{}
	renderExportChildren(state, n, ctx)
	out := strings.TrimSpace(state.sb.String())
	state.sb = saved
	return out
}

// registerAttachment records filename as referenced by the page body, so the
// caller downloads it. Deduplicates — the same attachment is often linked
// more than once.
func registerAttachment(state *confluenceExportState, filename string) {
	if filename == "" || state.seenAttach[filename] {
		return
	}
	state.seenAttach[filename] = true
	state.attachment = append(state.attachment, filename)
}

// --- DOM helpers ---

// findDescendant returns the first descendant of n with the given tag name,
// depth-first, or nil if none exists.
func findDescendant(n *html.Node, tag string) *html.Node {
	if n == nil {
		return nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
		if found := findDescendant(c, tag); found != nil {
			return found
		}
	}
	return nil
}

// collectTag returns every descendant of n with the given tag name, in
// document order.
func collectTag(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == tag {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(n)
	return out
}

// attrVal returns the value of n's attribute named key, or "" if absent.
func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textContent concatenates all text within n's subtree, ignoring markup.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// plainTextBodyContent returns the raw text of an <ac:plain-text-body> (or
// similar CDATA-wrapped element). The HTML parser surfaces CDATA sections as
// comment nodes since storage-format XHTML isn't valid foreign-content HTML,
// so the "[CDATA[" ... "]]" wrapper is stripped back off here.
func plainTextBodyContent(n *html.Node) string {
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.CommentNode:
			sb.WriteString(strings.TrimSuffix(strings.TrimPrefix(c.Data, "[CDATA["), "]]"))
		case html.TextNode:
			sb.WriteString(c.Data)
		}
	}
	return sb.String()
}

// macroParam returns the text value of a structured macro's <ac:parameter
// ac:name="paramName">, or "" if not present.
func macroParam(n *html.Node, paramName string) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "ac:parameter" && attrVal(c, "ac:name") == paramName {
			return textContent(c)
		}
	}
	return ""
}

// exportImageExtensions lists file extensions rendered as markdown images
// (![]()) rather than plain links when referenced from an unrecognised macro.
var exportImageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".svg": true, ".bmp": true,
}

// isImageFilename reports whether name has a common image extension.
func isImageFilename(name string) bool {
	return exportImageExtensions[strings.ToLower(filepath.Ext(name))]
}

// mdEscapeCell makes text safe to embed as a single markdown table cell:
// collapse embedded newlines (markdown table cells can't span lines) and
// escape pipe characters so they don't get read as extra column separators.
func mdEscapeCell(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	return strings.ReplaceAll(text, "|", `\|`)
}
