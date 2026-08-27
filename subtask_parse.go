package main

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// parsedTask holds a single task extracted from a markdown list or heading.
type parsedTask struct {
	Summary string
}

// parseOpts controls how tasks are extracted from markdown.
type parseOpts struct {
	// HeadingPattern extracts tasks from headings matching this regex.
	// When set, list items are ignored. Example: `^Task \d+:` matches "### Task 1: Do something".
	HeadingPattern string
	// Section limits list extraction to the named section heading.
	// When set, only list items under this heading (until the next heading of
	// equal or higher level) are extracted. Example: "Initial Task List".
	Section string
}

// parseTasksFromMarkdown extracts task summaries from a markdown file.
// Default mode: extracts all list items (bullet, numbered, checkbox).
// With opts.HeadingPattern: extracts headings matching the pattern.
// With opts.Section: extracts list items only within the named section.
func parseTasksFromMarkdown(md string, opts parseOpts) []parsedTask {
	src := []byte(md)
	parser := goldmark.New(
		goldmark.WithExtensions(extension.TaskList),
	).Parser()
	root := parser.Parse(text.NewReader(src))

	if opts.HeadingPattern != "" {
		return extractHeadingTasks(root, src, opts.HeadingPattern)
	}
	if opts.Section != "" {
		return extractSectionListItems(root, src, opts.Section)
	}

	var tasks []parsedTask
	collectListItems(root, src, &tasks)
	return tasks
}

// extractHeadingTasks finds headings whose text matches the given regex pattern
// and returns each as a task. The matched prefix is stripped from the summary
// (e.g. "Task 1: " is removed, leaving just the title).
func extractHeadingTasks(root ast.Node, src []byte, pattern string) []parsedTask {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	var tasks []parsedTask
	for c := root.FirstChild(); c != nil; c = c.NextSibling() {
		h, ok := c.(*ast.Heading)
		if !ok {
			continue
		}
		text := extractHeadingText(h, src)
		if !re.MatchString(text) {
			continue
		}
		// Strip the matched prefix to produce a cleaner summary.
		summary := strings.TrimSpace(re.ReplaceAllString(text, ""))
		if summary == "" {
			summary = text
		}
		tasks = append(tasks, parsedTask{Summary: summary})
	}
	return tasks
}

// extractHeadingText returns the plain text of a heading node.
func extractHeadingText(h *ast.Heading, src []byte) string {
	var sb strings.Builder
	collectInlineText(h, src, &sb)
	return strings.TrimSpace(sb.String())
}

// extractSectionListItems finds a heading matching the section name and
// extracts list items from under it, stopping at the next heading of
// equal or higher level.
func extractSectionListItems(root ast.Node, src []byte, section string) []parsedTask {
	sectionLower := strings.ToLower(section)
	inSection := false
	sectionLevel := 0
	var tasks []parsedTask

	for c := root.FirstChild(); c != nil; c = c.NextSibling() {
		if h, ok := c.(*ast.Heading); ok {
			headingText := strings.ToLower(extractHeadingText(h, src))
			if !inSection && strings.Contains(headingText, sectionLower) {
				inSection = true
				sectionLevel = h.Level
				continue
			}
			if inSection && h.Level <= sectionLevel {
				break // Next section at same or higher level — stop.
			}
		}
		if inSection {
			collectListItems(c, src, &tasks)
		}
	}
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
			collectNestedLists(c, src, tasks)
		} else {
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
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		collectInlineText(c, src, sb)
	}
}

// cleanTaskSummary trims whitespace and strips leading checkbox markers.
func cleanTaskSummary(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"[ ] ", "[x] ", "[X] "} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}
	return s
}
