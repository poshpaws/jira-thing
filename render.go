package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"jira-thing/internal/api"
)

// renderLastComment prints header and glamour-rendered body of a Jira comment.
func renderLastComment(comment api.Comment) {
	author := getString(comment.Author, "displayName")
	date := comment.Created
	if len(date) >= 10 {
		date = date[:10]
	}
	fmt.Printf("Last comment by %s on %s:\n\n", author, date)

	md := adfToMarkdown(comment.Body)
	rendered, err := glamour.Render(md, "dark")
	if err != nil {
		fmt.Print(md)
		return
	}
	fmt.Print(rendered)
}

// adfToMarkdown converts an Atlassian Document Format node tree to markdown.
func adfToMarkdown(node map[string]any) string {
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
		return prefixLines(joinChildren(node), "> ")
	case "mention":
		return adfMention(node)
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

// joinChildren concatenates adfToMarkdown output for all child nodes.
func joinChildren(node map[string]any) string {
	var sb strings.Builder
	for _, child := range adfChildren(node) {
		sb.WriteString(adfToMarkdown(child))
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
		}
	}
	return text
}

// renderBulletItems renders an ADF bulletList node as markdown bullet list.
func renderBulletItems(node map[string]any) string {
	var sb strings.Builder
	for _, item := range adfChildren(node) {
		sb.WriteString("- " + strings.TrimRight(joinChildren(item), "\n") + "\n")
	}
	return sb.String()
}

// renderOrderedItems renders an ADF orderedList node as markdown numbered list.
func renderOrderedItems(node map[string]any) string {
	var sb strings.Builder
	for i, item := range adfChildren(node) {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, strings.TrimRight(joinChildren(item), "\n"))
	}
	return sb.String()
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
	return strings.Repeat("#", int(level)) + " " + joinChildren(node) + "\n\n"
}

// adfCodeBlock converts an ADF codeBlock node to a fenced markdown code block.
func adfCodeBlock(node map[string]any) string {
	attrs, _ := node["attrs"].(map[string]any)
	lang, _ := attrs["language"].(string)
	return "```" + lang + "\n" + joinChildren(node) + "```\n\n"
}

// adfMention extracts the display name from an ADF mention node.
func adfMention(node map[string]any) string {
	attrs, _ := node["attrs"].(map[string]any)
	name, _ := attrs["text"].(string)
	return "@" + name
}
