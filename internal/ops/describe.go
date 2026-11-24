package ops

import (
	"fmt"
	"strings"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
	"jira-thing/internal/termsafe"
)

// MaxDescribeComments caps how many comments describe fetches.
const MaxDescribeComments = 100

// DefaultDescribeComments is how many comments describe shows when not told:
// the latest one, so a ticket's current state is visible at a glance.
const DefaultDescribeComments = 1

// DescribeResult is a ticket plus its newest comments, oldest first.
// CommentTotal counts every comment on the ticket.
type DescribeResult struct {
	Issue        map[string]any
	Comments     []api.Comment
	CommentTotal int
}

// DescribeTicket fetches a ticket and, when comments > 0, its newest comments.
func DescribeTicket(conn api.JiraConnection, key string, comments int) (DescribeResult, error) {
	if comments < 0 || comments > MaxDescribeComments {
		return DescribeResult{}, fmt.Errorf("comments must be between 0 and %d", MaxDescribeComments)
	}
	issue, err := api.FetchIssue(conn, key)
	if err != nil {
		return DescribeResult{}, fmt.Errorf("fetching %s: %w", key, err)
	}
	res := DescribeResult{Issue: issue}
	if comments == 0 {
		return res, nil
	}
	page, err := api.FetchRecentComments(conn, key, comments)
	if err != nil {
		return DescribeResult{}, fmt.Errorf("fetching comments on %s: %w", key, err)
	}
	res.Comments, res.CommentTotal = page.Comments, page.Total
	if res.Comments == nil {
		res.Comments = []api.Comment{}
	}
	return res, nil
}

// CommentsMarkdown renders the fetched comments as a markdown section, which
// both front ends append to their ticket view. Each body is a quoted block.
// It is empty when none were asked for or the ticket has none.
func CommentsMarkdown(res DescribeResult) string {
	if len(res.Comments) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n## Comments (latest %d of %d)\n\n", len(res.Comments), res.CommentTotal)
	for _, c := range res.Comments {
		author := termsafe.Line(str(c.Author, "displayName"))
		fmt.Fprintf(&sb, "### %s — %s\n\n%s\n\n", author, termsafe.Line(shortTimestamp(c.Created)), QuoteBlock(adf.ToMarkdown(c.Body)))
	}
	return sb.String()
}

// QuoteBlock cleans text written by Jira users and quotes every line with
// "> ", so it can't forge headings or other structure around it.
func QuoteBlock(md string) string {
	lines := strings.Split(strings.TrimRight(termsafe.Clean(md), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight("> "+l, " ")
	}
	return strings.Join(lines, "\n")
}

// shortTimestamp trims a Jira timestamp to "YYYY-MM-DD HH:MM".
func shortTimestamp(ts string) string {
	const n = len("2006-01-02T15:04")
	if len(ts) < n {
		return ts
	}
	return strings.Replace(ts[:n], "T", " ", 1)
}
