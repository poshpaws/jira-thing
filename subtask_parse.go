package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// parsedTask holds a single task extracted from a markdown list or heading.
type parsedTask struct {
	Summary     string
	Description string // markdown body text (heading mode only)
}

// parseOpts controls how tasks are extracted from markdown.
type parseOpts struct {
	// HeadingPattern extracts tasks from headings matching this regex.
	// When set, list items are ignored. The body text between matched headings
	// is captured as the task description.
	// Example: `^Task \d+:` matches "### Task 1: Do something".
	HeadingPattern string
	// Section limits list extraction to the named section heading.
	// When set, only list items under this heading (until the next heading of
	// equal or higher level) are extracted. Example: "Initial Task List".
	Section string
}

// parseTasksFromMarkdown extracts task summaries from a markdown file.
// Default mode: extracts all list items (bullet, numbered, checkbox).
// With opts.HeadingPattern: extracts headings matching the pattern, with body as description.
// With opts.Section: extracts list items only within the named section.
func parseTasksFromMarkdown(md string, opts parseOpts) []parsedTask {
	src := []byte(md)
	parser := goldmark.New(
		goldmark.WithExtensions(extension.TaskList),
	).Parser()
	root := parser.Parse(text.NewReader(src))

	if opts.HeadingPattern != "" {
		return extractHeadingTasks(root, src, md, opts.HeadingPattern)
	}
	if opts.Section != "" {
		return extractSectionListItems(root, src, opts.Section)
	}

	var tasks []parsedTask
	collectListItems(root, src, &tasks)
	return tasks
}

// headingSpan records the position and metadata of a matched heading.
type headingSpan struct {
	summary  string
	level    int
	bodyFrom int // byte offset where the body starts (after the heading line)
}

// extractHeadingTasks finds headings whose text matches the given regex pattern.
// Each matched heading becomes a task with its summary set to the heading text
// (with the regex match stripped) and its description set to the markdown body
// between the heading and the next heading of equal or higher level.
func extractHeadingTasks(root ast.Node, src []byte, md string, pattern string) []parsedTask {
	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid --heading regex %q: %v\n", pattern, err)
		return nil
	}

	// First pass: find all matched headings and record their byte positions.
	var spans []headingSpan
	for c := root.FirstChild(); c != nil; c = c.NextSibling() {
		h, ok := c.(*ast.Heading)
		if !ok {
			continue
		}
		headingText := extractHeadingText(h, src)
		if !re.MatchString(headingText) {
			continue
		}
		summary := strings.TrimSpace(re.ReplaceAllString(headingText, ""))
		if summary == "" {
			summary = headingText
		}
		// Body starts after the last line of the heading node.
		bodyStart := nodeEndOffset(h, src)
		spans = append(spans, headingSpan{summary: summary, level: h.Level, bodyFrom: bodyStart})
	}

	// Second pass: determine body end for each span and extract the markdown.
	tasks := make([]parsedTask, len(spans))
	for i, span := range spans {
		bodyEnd := findBodyEnd(root, src, span.bodyFrom, span.level)
		body := strings.TrimSpace(md[span.bodyFrom:bodyEnd])
		// Strip leading "---" horizontal rules that separate tasks.
		body = stripLeadingRule(body)
		tasks[i] = parsedTask{Summary: span.summary, Description: body}
	}
	return tasks
}

// findBodyEnd scans forward from startOffset through AST nodes to find where
// the body of a heading section ends — at the next heading of equal or higher
// level, or at the end of the document.
func findBodyEnd(root ast.Node, src []byte, startOffset, level int) int {
	for c := root.FirstChild(); c != nil; c = c.NextSibling() {
		h, ok := c.(*ast.Heading)
		if !ok {
			continue
		}
		hStart := nodeStartOffset(h, src)
		if hStart <= startOffset {
			continue
		}
		if h.Level <= level {
			return hStart
		}
	}
	return len(src)
}

// nodeStartOffset returns the byte offset where a node begins in the source.
func nodeStartOffset(n ast.Node, src []byte) int {
	lines := n.Lines()
	if lines.Len() > 0 {
		return lines.At(0).Start
	}
	// Fallback for nodes without lines: walk to first text child.
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			return t.Segment.Start
		}
	}
	return len(src)
}

// nodeEndOffset returns the byte offset just past the last line of a node.
func nodeEndOffset(n ast.Node, src []byte) int {
	lines := n.Lines()
	if lines.Len() > 0 {
		return lines.At(lines.Len() - 1).Stop
	}
	// Fallback: find the end of the last inline child.
	var end int
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok && t.Segment.Stop > end {
			end = t.Segment.Stop
		}
	}
	// Skip past the trailing newline.
	if end < len(src) && src[end] == '\n' {
		end++
	}
	return end
}

// stripLeadingRule removes a leading horizontal rule (---) from the body text.
func stripLeadingRule(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "---") {
		s = strings.TrimSpace(s[3:])
	}
	return s
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
				break
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
// excluding any nested lists (which are handled separately).
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
