package api

import (
	"fmt"
	"strings"
)

// DedupeMaxResults caps the candidates FindOpenIssueBySummary examines. A full
// page with no exact match is reported as an error, since the match could be
// on a later page.
const DedupeMaxResults = 100

// jqlEscaper escapes the two characters special inside a quoted JQL string.
var jqlEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// FindOpenIssueBySummary returns the key of an unresolved issue in project
// whose summary equals summary, ignoring case and surrounding whitespace, or
// "" if there is none. project is a key or ID. JQL's "~" is a fuzzy text
// match, so it only narrows the candidates; the comparison here decides. An
// error means the check was inconclusive, not that no duplicate exists.
func FindOpenIssueBySummary(conn JiraConnection, project, summary string) (string, error) {
	result, err := SearchIssues(conn, SearchQuery{
		JQL:        openSummaryJQL(project, summary),
		Fields:     []string{"summary"},
		MaxResults: DedupeMaxResults,
	})
	if err != nil {
		return "", fmt.Errorf("searching for duplicates: %w", err)
	}
	for _, issue := range result.Issues {
		if sameSummary(issueSummary(issue), summary) {
			key, _ := issue["key"].(string)
			return key, nil
		}
	}
	if len(result.Issues) >= DedupeMaxResults {
		return "", fmt.Errorf("searching for duplicates: over %d similar open tickets in %s, too many to check", DedupeMaxResults, project)
	}
	return "", nil
}

// openSummaryJQL builds the candidate search. The summary is quoted twice: the
// inner quotes make it a phrase for the text index, which also leaves its
// punctuation to the analyser rather than the query parser, and the outer
// quotes make that phrase a JQL string.
func openSummaryJQL(project, summary string) string {
	return fmt.Sprintf("project = %s AND summary ~ %s AND statusCategory != Done ORDER BY created ASC",
		jqlQuote(project), jqlQuote(jqlQuote(summary)))
}

// jqlQuote renders s as a quoted JQL string.
func jqlQuote(s string) string {
	return `"` + jqlEscaper.Replace(s) + `"`
}

// sameSummary reports whether two summaries match, ignoring case and surrounding whitespace.
func sameSummary(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// issueSummary extracts fields.summary from a search result issue.
func issueSummary(issue map[string]any) string {
	fields, _ := issue["fields"].(map[string]any)
	summary, _ := fields["summary"].(string)
	return summary
}
