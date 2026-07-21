package main

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// markdownToADF converts markdown text to an Atlassian Document Format doc node.
// It is the inverse of adfToMarkdown in render.go.
func markdownToADF(md string) map[string]any {
	src := []byte(md)
	parser := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough),
	).Parser()
	root := parser.Parse(text.NewReader(src))
	return map[string]any{"type": "doc", "version": 1, "content": convertBlocks(root, src)}
}

// adfNode builds a generic ADF node with the given type and children.
func adfNode(typ string, content []any) map[string]any {
	return map[string]any{"type": typ, "content": content}
}

// adfText builds an ADF text node, attaching marks only when present.
func adfText(s string, marks []any) map[string]any {
	n := map[string]any{"type": "text", "text": s}
	if len(marks) > 0 {
		n["marks"] = marks
	}
	return n
}

// convertBlocks converts all block-level children of parent to ADF nodes.
func convertBlocks(parent ast.Node, src []byte) []any {
	blocks := []any{}
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		if n := convertBlock(c, src); n != nil {
			blocks = append(blocks, n)
		}
	}
	return blocks
}

// convertBlock converts a single goldmark block node to its ADF equivalent.
// Unknown block types are dropped.
func convertBlock(n ast.Node, src []byte) map[string]any {
	switch node := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return adfNode("paragraph", convertInlines(n, src, nil))
	case *ast.Heading:
		h := adfNode("heading", convertInlines(n, src, nil))
		h["attrs"] = map[string]any{"level": node.Level}
		return h
	case *ast.List:
		return convertList(node, src)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		return convertCodeBlock(n, src)
	case *ast.Blockquote:
		return adfNode("blockquote", convertBlocks(n, src))
	case *ast.ThematicBreak:
		return map[string]any{"type": "rule"}
	case *east.Table:
		return adfNode("table", convertTableRows(node, src))
	default:
		return nil
	}
}

// convertList converts a goldmark list to an ADF bulletList or orderedList.
func convertList(list *ast.List, src []byte) map[string]any {
	items := []any{}
	for c := list.FirstChild(); c != nil; c = c.NextSibling() {
		items = append(items, adfNode("listItem", convertBlocks(c, src)))
	}
	if list.IsOrdered() {
		n := adfNode("orderedList", items)
		n["attrs"] = map[string]any{"order": list.Start}
		return n
	}
	return adfNode("bulletList", items)
}

// convertCodeBlock converts a goldmark code block to an ADF codeBlock,
// carrying the fence language as an attribute when present.
func convertCodeBlock(n ast.Node, src []byte) map[string]any {
	content := []any{}
	if code := rawLines(n, src); code != "" {
		content = append(content, adfText(code, nil))
	}
	block := adfNode("codeBlock", content)
	if fenced, ok := n.(*ast.FencedCodeBlock); ok {
		if lang := fenced.Language(src); len(lang) > 0 {
			block["attrs"] = map[string]any{"language": string(lang)}
		}
	}
	return block
}

// rawLines joins the source line segments of a block node into one string.
func rawLines(n ast.Node, src []byte) string {
	var sb strings.Builder
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		sb.Write(seg.Value(src))
	}
	return sb.String()
}

// convertTableRows converts table rows, using tableHeader cells for the header row.
func convertTableRows(table *east.Table, src []byte) []any {
	rows := []any{}
	for r := table.FirstChild(); r != nil; r = r.NextSibling() {
		cellType := "tableCell"
		if _, ok := r.(*east.TableHeader); ok {
			cellType = "tableHeader"
		}
		rows = append(rows, adfNode("tableRow", convertTableCells(r, src, cellType)))
	}
	return rows
}

// convertTableCells converts each cell of a row, wrapping inline content in a paragraph.
func convertTableCells(row ast.Node, src []byte, cellType string) []any {
	cells := []any{}
	for c := row.FirstChild(); c != nil; c = c.NextSibling() {
		para := adfNode("paragraph", convertInlines(c, src, nil))
		cells = append(cells, adfNode(cellType, []any{para}))
	}
	return cells
}

// convertInlines converts all inline children of parent, applying accumulated marks.
func convertInlines(parent ast.Node, src []byte, marks []any) []any {
	out := []any{}
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, convertInline(c, src, marks)...)
	}
	return out
}

// convertInline converts one inline node to ADF text nodes with marks.
// Container inlines (emphasis, links, strikethrough) recurse with an added mark.
func convertInline(n ast.Node, src []byte, marks []any) []any {
	switch node := n.(type) {
	case *ast.Text:
		return textNodes(node, src, marks)
	case *ast.String:
		return []any{adfText(string(node.Value), marks)}
	case *ast.Emphasis:
		return convertInlines(n, src, appendMark(marks, emphasisMark(node)))
	case *ast.CodeSpan:
		return []any{adfText(rawText(n, src), appendMark(marks, markNode("code")))}
	case *east.Strikethrough:
		return convertInlines(n, src, appendMark(marks, markNode("strike")))
	case *ast.Link:
		return convertInlines(n, src, appendMark(marks, linkMark(string(node.Destination))))
	case *ast.AutoLink:
		url := string(node.URL(src))
		return []any{adfText(url, appendMark(marks, linkMark(url)))}
	default:
		return convertInlines(n, src, marks)
	}
}

// textNodes converts an ast.Text segment plus its trailing soft/hard line break.
func textNodes(t *ast.Text, src []byte, marks []any) []any {
	out := []any{}
	if s := string(t.Segment.Value(src)); s != "" {
		out = append(out, adfText(s, marks))
	}
	if t.HardLineBreak() {
		out = append(out, map[string]any{"type": "hardBreak"})
	} else if t.SoftLineBreak() {
		out = append(out, adfText(" ", marks))
	}
	return out
}

// rawText concatenates the raw text of an inline node's children (for code spans).
func rawText(n ast.Node, src []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			sb.Write(t.Segment.Value(src))
		}
	}
	return sb.String()
}

// appendMark returns a copy of marks with one more mark appended.
// Copying avoids sibling nodes sharing a backing array.
func appendMark(marks []any, mark map[string]any) []any {
	out := make([]any, len(marks), len(marks)+1)
	copy(out, marks)
	return append(out, mark)
}

// markNode builds a simple ADF mark of the given type.
func markNode(typ string) map[string]any { return map[string]any{"type": typ} }

// emphasisMark maps goldmark emphasis level to the ADF em or strong mark.
func emphasisMark(node *ast.Emphasis) map[string]any {
	if node.Level == 2 {
		return markNode("strong")
	}
	return markNode("em")
}

// linkMark builds an ADF link mark pointing at href.
func linkMark(href string) map[string]any {
	return map[string]any{"type": "link", "attrs": map[string]any{"href": href}}
}
