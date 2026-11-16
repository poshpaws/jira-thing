package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"

	"github.com/charmbracelet/glamour"
	"github.com/mattn/go-isatty"
)

// shouldRenderPlain reports whether output should skip ANSI styling entirely —
// either because -plain/-raw was given, or because stdout isn't a terminal, so
// piping (e.g. "describe KEY | cat") never embeds escape codes.
func shouldRenderPlain(plain bool) bool {
	return plain || !isatty.IsTerminal(os.Stdout.Fd())
}

// renderMarkdown prints md glamour-rendered when writing to a terminal, or as
// plain text otherwise (piped output, or -plain).
func renderMarkdown(md string, plain bool) {
	md = termsafe.Clean(md)
	if shouldRenderPlain(plain) {
		fmt.Print(md)
		return
	}
	rendered, err := glamour.Render(md, "dark")
	if err != nil {
		fmt.Print(md)
		return
	}
	fmt.Print(rendered)
}

// renderLastComment prints header and body of a Jira comment: rendered
// markdown by default, or the stored ADF body verbatim with -raw.
func renderLastComment(comment api.Comment, plain, raw bool) {
	author := getString(comment.Author, "displayName")
	date := comment.Created
	if len(date) >= 10 {
		date = date[:10]
	}
	fmt.Printf("Last comment by %s on %s:\n\n", author, date)

	if raw {
		printRawJSON(comment.Body)
		return
	}
	renderMarkdown(adf.ToMarkdown(comment.Body), plain)
}

// renderDescribe prints a Jira issue: rendered markdown by default (key,
// summary, core fields, description), or the raw issue JSON with -raw.
func renderDescribe(issue map[string]any, plain, raw bool) {
	if raw {
		printRawJSON(issue)
		return
	}
	renderMarkdown(buildDescribeMarkdown(issue), plain)
}

// renderDescribeResult prints a ticket followed by its fetched comments, if
// any were asked for; with raw, both as one JSON object.
func renderDescribeResult(res ops.DescribeResult, plain, raw bool) {
	switch {
	case res.Comments == nil:
		renderDescribe(res.Issue, plain, raw)
	case raw:
		printRawJSON(map[string]any{"issue": res.Issue, "comments": res.Comments, "commentTotal": res.CommentTotal})
	default:
		renderMarkdown(buildDescribeMarkdown(res.Issue)+ops.CommentsMarkdown(res), plain)
	}
}

// printRawJSON pretty-prints v as JSON, falling back to a plain error note if
// it somehow doesn't marshal.
func printRawJSON(v any) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("<failed to render raw JSON: %v>\n", err)
		return
	}
	fmt.Println(string(body))
}

// buildDescribeMarkdown renders a Jira issue's key fields and description as markdown.
func buildDescribeMarkdown(issue map[string]any) string {
	f, _ := issue["fields"].(map[string]any)
	if f == nil {
		f = map[string]any{}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s: %s\n\n", getString(issue, "key"), getString(f, "summary"))
	fmt.Fprintf(&sb, "- **Status:** %s\n", nestedString(f, "status", "name"))
	fmt.Fprintf(&sb, "- **Priority:** %s\n", nestedString(f, "priority", "name"))
	fmt.Fprintf(&sb, "- **Assignee:** %s\n", describePersonOr(f, "assignee", "Unassigned"))
	fmt.Fprintf(&sb, "- **Reporter:** %s\n", describePersonOr(f, "reporter", "Unknown"))
	fmt.Fprintf(&sb, "- **Created:** %s\n", shortDate(getString(f, "created")))
	fmt.Fprintf(&sb, "- **Updated:** %s\n\n", shortDate(getString(f, "updated")))
	writeLinkedIssues(&sb, api.ParseIssueLinks(issue))
	sb.WriteString("## Description\n\n")
	if desc, ok := f["description"].(map[string]any); ok {
		sb.WriteString(adf.ToMarkdown(desc))
	} else {
		sb.WriteString("_No description._\n")
	}
	return sb.String()
}

// writeLinkedIssues appends a "Linked Tickets" section, or nothing if there are no links.
func writeLinkedIssues(sb *strings.Builder, links []api.LinkedIssue) {
	if len(links) == 0 {
		return
	}
	sb.WriteString("## Linked Tickets\n\n")
	for _, link := range links {
		fmt.Fprintf(sb, "- %s\n", link)
	}
	sb.WriteString("\n")
}

// describePersonOr returns the displayName of a nested person field, or fallback if absent.
func describePersonOr(f map[string]any, key, fallback string) string {
	name := nestedString(f, key, "displayName")
	if name == "" {
		return fallback
	}
	return name
}

// shortDate truncates an ISO timestamp to its date portion.
func shortDate(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}
