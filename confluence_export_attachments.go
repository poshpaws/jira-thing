package main

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// panelLabels maps a Confluence ADF panel macro name to the bold label used
// when it's rendered as a markdown blockquote (markdown has no native
// equivalent of Confluence's coloured info/note/warning/tip panels).
var panelLabels = map[string]string{
	"info": "ℹ️ **Info:**", "note": "📝 **Note:**",
	"warning": "⚠️ **Warning:**", "tip": "💡 **Tip:**", "panel": "",
}

// renderExportImage renders <ac:image>: an external <ri:url> becomes a
// markdown image pointing at that URL; a local <ri:attachment> becomes a
// markdown image pointing at the downloaded copy, and the filename is
// recorded for download.
func renderExportImage(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	alt := attrVal(n, "ac:alt")
	if u := findDescendant(n, "ri:url"); u != nil {
		fmt.Fprintf(&state.sb, "![%s](%s)", alt, attrVal(u, "ri:value"))
		return
	}
	if att := findDescendant(n, "ri:attachment"); att != nil {
		renderAttachmentRef(state, attrVal(att, "ri:filename"), alt, ctx.attachmentDir, true)
	}
}

// renderExportLink renders <ac:link>: a link to an attachment becomes a
// markdown link to the downloaded copy; a link to another Confluence page is
// rendered as bold text (its title is known, but resolving a URL would need
// an extra API call this pure converter doesn't make); anything else falls
// back to its link body text.
func renderExportLink(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	if att := findDescendant(n, "ri:attachment"); att != nil {
		filename := attrVal(att, "ri:filename")
		renderAttachmentRef(state, filename, linkBodyText(n, filename), ctx.attachmentDir, false)
		return
	}
	if page := findDescendant(n, "ri:page"); page != nil {
		title := attrVal(page, "ri:content-title")
		fmt.Fprintf(&state.sb, "**%s**", linkBodyText(n, title))
		return
	}
	state.sb.WriteString(linkBodyText(n, ""))
}

// linkBodyText extracts an <ac:link>'s display text from its
// ac:plain-text-link-body or ac:link-body child, falling back to fallback.
func linkBodyText(n *html.Node, fallback string) string {
	if body := findDescendant(n, "ac:plain-text-link-body"); body != nil {
		return plainTextBodyContent(body)
	}
	if body := findDescendant(n, "ac:link-body"); body != nil {
		return strings.TrimSpace(textContent(body))
	}
	return fallback
}

// renderAttachmentRef emits a markdown reference to a page attachment and
// records the filename so the caller downloads it. Used for images, plain
// attachment links, and macro-embedded documents alike — they all resolve
// to the same underlying ri:attachment reference.
func renderAttachmentRef(state *confluenceExportState, filename, text, attachmentDir string, asImage bool) {
	if filename == "" {
		return
	}
	registerAttachment(state, filename)
	if text == "" {
		text = filename
	}
	path := filename
	if attachmentDir != "" {
		path = attachmentDir + "/" + filename
	}
	prefix := ""
	if asImage {
		prefix = "!"
	}
	fmt.Fprintf(&state.sb, "%s[%s](%s)", prefix, text, path)
}

// renderExportMacro dispatches a <ac:structured-macro> by its ac:name.
func renderExportMacro(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	switch name := attrVal(n, "ac:name"); name {
	case "code":
		renderExportCodeMacro(state, n)
	case "info", "note", "warning", "tip", "panel":
		renderExportPanelMacro(state, n, ctx, name)
	case "drawio":
		renderExportDrawioMacro(state, n, ctx)
	default:
		renderExportGenericMacro(state, n, ctx)
	}
}

// renderExportCodeMacro renders the Confluence "code" macro as a fenced
// markdown code block, preserving the declared language where set.
func renderExportCodeMacro(state *confluenceExportState, n *html.Node) {
	lang := macroParam(n, "language")
	body := ""
	if pt := findDescendant(n, "ac:plain-text-body"); pt != nil {
		body = plainTextBodyContent(pt)
	}
	fmt.Fprintf(&state.sb, "```%s\n%s\n```\n\n", lang, strings.TrimRight(body, "\n"))
}

// renderExportPanelMacro renders an info/note/warning/tip/panel macro as a
// labelled markdown blockquote.
func renderExportPanelMacro(state *confluenceExportState, n *html.Node, ctx renderCtx, name string) {
	body := findDescendant(n, "ac:rich-text-body")
	if body == nil {
		body = n
	}
	inner := renderSubtree(state, body, ctx)
	if inner == "" {
		return
	}
	writeQuoted(state, inner, panelLabels[name])
	state.sb.WriteString("\n")
}

// renderExportDrawioMacro renders a draw.io diagram macro as a reference to
// its attached .drawio source (Confluence's native drawio renderer has no
// markdown equivalent, so the source file is downloaded instead).
func renderExportDrawioMacro(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	name := macroParam(n, "diagramName")
	renderAttachmentRef(state, name+".drawio", "diagram: "+name, ctx.attachmentDir, false)
	state.sb.WriteString("\n\n")
}

// renderExportGenericMacro handles any structured macro without dedicated
// support (view-file, multimedia, attachments, expand, status, jira, ...).
// If it embeds a ri:attachment — the common case for "embedded document"
// macros like view-file/multimedia — it's rendered as an attachment
// reference so the document is still downloaded and linked. Otherwise its
// rich-text body (or plain text as a last resort) is rendered so content
// isn't silently dropped.
func renderExportGenericMacro(state *confluenceExportState, n *html.Node, ctx renderCtx) {
	if att := findDescendant(n, "ri:attachment"); att != nil {
		filename := attrVal(att, "ri:filename")
		renderAttachmentRef(state, filename, "", ctx.attachmentDir, isImageFilename(filename))
		state.sb.WriteString("\n\n")
		return
	}
	if body := findDescendant(n, "ac:rich-text-body"); body != nil {
		renderExportChildren(state, body, ctx)
		return
	}
	if txt := strings.TrimSpace(textContent(n)); txt != "" {
		fmt.Fprintf(&state.sb, "%s\n\n", txt)
	}
}
