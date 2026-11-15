package adf

import (
	"fmt"
	"strings"
	"unicode"
)

// ToMarkdown converts an Atlassian Document Format node tree to markdown.
func ToMarkdown(node map[string]any) string {
	switch getString(node, "type") {
	case "doc", "listItem":
		return joinChildren(node)
	case "paragraph":
		return joinChildren(node) + "\n\n"
	case "text":
		return applyMarks(getString(node, "text"), node["marks"])
	case "hardBreak":
		return "  \n"
	case "heading":
		return adfHeading(node)
	case "bulletList":
		return renderBulletItems(node) + "\n"
	case "orderedList":
		return renderOrderedItems(node) + "\n"
	case "codeBlock":
		return adfCodeBlock(node)
	case "blockquote":
		return prefixLines(joinChildren(node), "> ") + "\n"
	case "mention":
		return adfMention(node)
	case "rule":
		return "---\n\n"
	case "table":
		return renderTable(node)
	default:
		return joinChildren(node)
	}
}

// adfChildren returns the typed content slice of an ADF node.
func adfChildren(node map[string]any) []map[string]any {
	content, ok := node["content"].([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(content))
	for _, c := range content {
		if m, ok := c.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

// joinChildren concatenates ToMarkdown output for all child nodes.
func joinChildren(node map[string]any) string {
	var sb strings.Builder
	for _, child := range adfChildren(node) {
		sb.WriteString(ToMarkdown(child))
	}
	return sb.String()
}

// applyMarks wraps text in markdown syntax for strong, em, code, and strike marks.
func applyMarks(text string, rawMarks any) string {
	marks, ok := rawMarks.([]any)
	if !ok {
		return text
	}
	for _, m := range marks {
		mark, ok := m.(map[string]any)
		if !ok {
			continue
		}
		switch mark["type"] {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "_" + text + "_"
		case "code":
			text = "`" + text + "`"
		case "strike":
			text = "~~" + text + "~~"
		case "link":
			text = "[" + text + "](" + nestedString(mark, "attrs", "href") + ")"
		}
	}
	return text
}

// renderBulletItems renders an ADF bulletList node as markdown bullet list.
func renderBulletItems(node map[string]any) string {
	var sb strings.Builder
	for _, item := range adfChildren(node) {
		sb.WriteString(renderListItem("- ", item))
	}
	return sb.String()
}

// renderOrderedItems renders an ADF orderedList node as markdown numbered list.
func renderOrderedItems(node map[string]any) string {
	var sb strings.Builder
	for i, item := range adfChildren(node) {
		sb.WriteString(renderListItem(fmt.Sprintf("%d. ", i+1), item))
	}
	return sb.String()
}

// renderListItem renders one ADF listItem, indenting nested content (such as
// sub-lists) to the marker width so it nests correctly in markdown.
func renderListItem(marker string, item map[string]any) string {
	content := strings.TrimRight(joinChildren(item), "\n")
	content = strings.ReplaceAll(content, "\n\n", "\n")
	indent := strings.Repeat(" ", len(marker))
	lines := strings.Split(content, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}
	return marker + strings.Join(lines, "\n") + "\n"
}

// prefixLines prepends prefix to every line of text.
func prefixLines(text, prefix string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(prefix + l + "\n")
	}
	return sb.String()
}

// adfHeading converts an ADF heading node to a markdown heading.
func adfHeading(node map[string]any) string {
	attrs, _ := node["attrs"].(map[string]any)
	level, _ := attrs["level"].(float64)
	if level < 1 || level > 6 {
		level = 1
	}
	return strings.Repeat("#", int(level)) + " " + joinChildren(node) + "\n\n"
}

// adfCodeBlock converts an ADF codeBlock node to a fenced markdown code block.
func adfCodeBlock(node map[string]any) string {
	attrs, _ := node["attrs"].(map[string]any)
	lang, _ := attrs["language"].(string)
	return "```" + lang + "\n" + joinChildren(node) + "```\n\n"
}

// renderTable converts an ADF table node to a markdown table, inserting the
// separator line after the first row.
func renderTable(node map[string]any) string {
	var sb strings.Builder
	for i, row := range adfChildren(node) {
		cells := renderTableCells(row)
		sb.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		if i == 0 {
			sb.WriteString("|" + strings.Repeat(" --- |", len(cells)) + "\n")
		}
	}
	return sb.String() + "\n"
}

// renderTableCells renders each cell of an ADF tableRow as trimmed inline text.
func renderTableCells(row map[string]any) []string {
	cells := []string{}
	for _, cell := range adfChildren(row) {
		cells = append(cells, strings.TrimSpace(joinChildren(cell)))
	}
	return cells
}

// adfMention extracts the display name from an ADF mention node.
func adfMention(node map[string]any) string {
	attrs, _ := node["attrs"].(map[string]any)
	name, _ := attrs["text"].(string)
	return "@" + name
}

// getString returns m[key] as a string, or "" if absent or not a string.
// Every string the renderer reads from ADF comes through here, so it strips
// terminal control characters: ticket text is written by other Jira users
// and must not be able to drive the reader's terminal with escape sequences.
func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return stripControl(v)
}

// stripControl removes C0 and C1 control characters (ESC, BEL, CSI, ...)
// except tab and newline, which neutralises ANSI and OSC escape sequences.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, s)
}

// nestedString returns m[key1][key2] as a string, or "" if any level is missing.
func nestedString(m map[string]any, key1, key2 string) string {
	if inner, ok := m[key1].(map[string]any); ok {
		return getString(inner, key2)
	}
	return ""
}
