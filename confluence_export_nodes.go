package main

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// exportElementFn renders one HTML element into state, given its render context.
type exportElementFn func(state *confluenceExportState, n *html.Node, ctx renderCtx)

// exportElementRenderers dispatches by tag name. A lookup table (rather than
// a large switch) keeps renderExportNode's cyclomatic complexity flat as tags
// are added, and lets several tag names share one handler (b/strong, i/em).
var exportElementRenderers map[string]exportElementFn

func init() {
	exportElementRenderers = map[string]exportElementFn{
		"h1": renderExportHeading, "h2": renderExportHeading, "h3": renderExportHeading,
		"h4": renderExportHeading, "h5": renderExportHeading, "h6": renderExportHeading,
		"p":                   renderExportParagraph,
		"ul":                  renderExportUnorderedList,
		"ol":                  renderExportOrderedList,
		"table":               renderExportTable,
		"blockquote":          renderExportBlockquote,
		"hr":                  renderExportRule,
		"br":                  renderExportBreak,
		"strong":              exportWrapped("**"),
		"b":                   exportWrapped("**"),
		"em":                  exportWrapped("*"),
		"i":                   exportWrapped("*"),
		"s":                   exportWrapped("~~"),
		"del":                 exportWrapped("~~"),
		"strike":              exportWrapped("~~"),
		"code":                renderExportInlineCode,
		"a":                   renderExportAnchor,
		"time":                renderExportTime,
		"ac:image":            renderExportImage,
		"ac:link":             renderExportLink,
		"ac:structured-macro": renderExportMacro,
	}
}

// --- headings, paragraphs, rules ---

func renderExportHeading(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	level := int(n.Data[1] - '0')
	state.sb.WriteString(strings.Repeat("#", level) + " ")
	renderExportChildren(state, n, ctx)
	state.sb.WriteString("\n\n")
}

// renderExportParagraph renders a paragraph, dropping ones that produced no
// output (Confluence pads pages with empty <p> spacers).
func renderExportParagraph(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	start := state.sb.Len()
	renderExportChildren(state, n, ctx)
	if state.sb.Len() == start {
		return
	}
	state.sb.WriteString("\n\n")
}

func renderExportRule(state *confluenceExportState, _ *html.Node, _ renderCtx) {
	state.sb.WriteString("---\n\n")
}

func renderExportBreak(state *confluenceExportState, _ *html.Node, _ renderCtx) {
	state.sb.WriteString("  \n")
}

func renderExportTime(state *confluenceExportState, n *html.Node, _ renderCtx) {
	state.sb.WriteString(attrVal(n, "datetime"))
}

// --- inline formatting ---

// exportWrapped returns a renderer that surrounds its children's markdown
// with a fixed marker — covers bold/italic/strikethrough, which differ only
// in the marker used.
func exportWrapped(mark string) exportElementFn {
	return func(state *confluenceExportState, n *html.Node, ctx renderCtx) {
		state.sb.WriteString(mark)
		renderExportChildren(state, n, ctx)
		state.sb.WriteString(mark)
	}
}

func renderExportInlineCode(state *confluenceExportState, n *html.Node, _ renderCtx) {
	fmt.Fprintf(&state.sb, "`%s`", textContent(n))
}

func renderExportAnchor(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	state.sb.WriteString("[")
	renderExportChildren(state, n, ctx)
	fmt.Fprintf(&state.sb, "](%s)", attrVal(n, "href"))
}

// --- lists ---

func renderExportUnorderedList(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	renderExportList(state, n, ctx, false)
}

func renderExportOrderedList(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	renderExportList(state, n, ctx, true)
}

func renderExportList(state *confluenceExportState, n *html.Node, ctx renderCtx, ordered bool) {
	itemCtx := renderCtx{attachmentDir: ctx.attachmentDir, listDepth: ctx.listDepth + 1}
	i := 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "li" {
			continue
		}
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", i)
			i++
		}
		renderExportListItem(state, c, itemCtx, marker)
	}
	if ctx.listDepth == 0 {
		state.sb.WriteString("\n")
	}
}

// renderExportListItem renders one <li>'s content as a standalone subtree,
// then prefixes its first line with the marker and continuation lines
// (including nested sub-lists) with matching indent.
func renderExportListItem(state *confluenceExportState, n *html.Node, ctx renderCtx, marker string) {
	indent := strings.Repeat("  ", ctx.listDepth-1)
	inner := renderSubtree(state, n, ctx)
	for i, line := range strings.Split(inner, "\n") {
		switch {
		case i == 0:
			fmt.Fprintf(&state.sb, "%s%s%s\n", indent, marker, line)
		case line != "":
			fmt.Fprintf(&state.sb, "%s  %s\n", indent, line)
		}
	}
}

// --- tables ---

func renderExportTable(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	rows := collectTag(n, "tr")
	for i, row := range rows {
		renderExportTableRow(state, row, ctx)
		if i == 0 {
			renderExportTableSeparator(state, row)
		}
	}
	if len(rows) > 0 {
		state.sb.WriteString("\n")
	}
}

func renderExportTableRow(state *confluenceExportState, row *html.Node, ctx renderCtx) {
	state.sb.WriteString("|")
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.Data != "td" && c.Data != "th") {
			continue
		}
		cell := mdEscapeCell(renderSubtree(state, c, renderCtx{attachmentDir: ctx.attachmentDir}))
		fmt.Fprintf(&state.sb, " %s |", cell)
	}
	state.sb.WriteString("\n")
}

func renderExportTableSeparator(state *confluenceExportState, headerRow *html.Node) {
	state.sb.WriteString("|")
	for c := headerRow.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			state.sb.WriteString(" --- |")
		}
	}
	state.sb.WriteString("\n")
}

// --- blockquotes and panels ---

func renderExportBlockquote(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	writeQuoted(state, renderSubtree(state, n, ctx), "")
	state.sb.WriteString("\n")
}

// writeQuoted writes text as a markdown blockquote, with an optional bold
// label prefixed to the first line — used for Confluence info/note/warning/
// tip panels, which have no direct markdown equivalent.
func writeQuoted(state *confluenceExportState, text, label string) {
	for i, line := range strings.Split(text, "\n") {
		if i == 0 && label != "" {
			fmt.Fprintf(&state.sb, "> %s %s\n", label, line)
			continue
		}
		fmt.Fprintf(&state.sb, "> %s\n", line)
	}
}
