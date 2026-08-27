package main

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// parsedTask holds a single task extracted from a markdown list.
type parsedTask struct {
	Summary string
}

// parseTasksFromMarkdown extracts task summaries from bullet lists, numbered
// lists, and checkbox lists in a markdown file. Nested items are flattened
// into the top-level list (Jira subtasks cannot have sub-subtasks).
// Only non-empty summaries are returned.
func parseTasksFromMarkdown(md string) []parsedTask {
	src := []byte(md)
	parser := goldmark.New(
		goldmark.WithExtensions(extension.TaskList),
	).Parser()
	root := parser.Parse(text.NewReader(src))

	var tasks []parsedTask
	collectListItems(root, src, &tasks)
	return tasks
}

// collectListItems walks the AST recursively and extracts text from every
// list item it encounters. Nested lists are flattened — their items appear
// after the parent item in order.
func collectListItems(n ast.Node, src []byte, tasks *[]parsedTask) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*ast.ListItem); ok {
			summary := extractListItemText(c, src)
			if summary != "" {
				*tasks = append(*tasks, parsedTask{Summary: summary})
			}
			// Recurse into nested lists inside this item.
			collectNestedLists(c, src, tasks)
		} else {
			// Keep walking into non-list-item nodes (document, lists, etc.)
			collectListItems(c, src, tasks)
		}
	}
}

// collectNestedLists finds any nested List nodes inside a ListItem and
// collects their items. This flattens one level of nesting.
func collectNestedLists(item ast.Node, src []byte, tasks *[]parsedTask) {
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*ast.List); ok {
			collectListItems(c, src, tasks)
		}
	}
}

// extractListItemText returns the plain text content of a list item,
// excluding any nested lists (which are handled separately). Checkbox
// markers like "[ ] " or "[x] " are stripped.
func extractListItemText(item ast.Node, src []byte) string {
	var sb strings.Builder
	for c := item.FirstChild(); c != nil; c = c.NextSibling() {
		// Skip nested lists — they become separate tasks.
		if _, ok := c.(*ast.List); ok {
			continue
		}
		collectInlineText(c, src, &sb)
	}
	return cleanTaskSummary(sb.String())
}

// collectInlineText recursively extracts raw text from inline AST nodes.
func collectInlineText(n ast.Node, src []byte, sb *strings.Builder) {
	if t, ok := n.(*ast.Text); ok {
		sb.Write(t.Segment.Value(src))
		if t.SoftLineBreak() || t.HardLineBreak() {
			sb.WriteByte(' ')
		}
		return
	}
	if s, ok := n.(*ast.String); ok {
		sb.Write(s.Value)
		return
	}
	// Recurse into container inlines (emphasis, links, code spans, etc.)
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		collectInlineText(c, src, sb)
	}
}

// cleanTaskSummary trims whitespace and strips leading checkbox markers.
func cleanTaskSummary(s string) string {
	s = strings.TrimSpace(s)
	// Strip checkbox markers: "[ ] ", "[x] ", "[X] "
	for _, prefix := range []string{"[ ] ", "[x] ", "[X] "} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}
	return s
}
