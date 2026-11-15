package ops

import (
	"fmt"
	"html"
	"jira-thing/internal/api"
	"jira-thing/internal/termsafe"
	"strings"
	"time"
)

// Toil page notes markers: content between them is user-written and kept across syncs.
const (
	notesStart = "<!-- jira-thing:notes:start -->"
	notesEnd   = "<!-- jira-thing:notes:end -->"
)

// extractNotes returns the content between note markers in a Confluence page body.
func extractNotes(body string) string {
	start := strings.Index(body, notesStart)
	end := strings.Index(body, notesEnd)
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return body[start+len(notesStart) : end]
}

// renderTicketPage builds a Confluence storage-format page for one Jira issue.
// The details section embeds the live Jira issue via the Confluence Jira macro.
// Existing notes are preserved; empty string produces a blank notes section.
func renderTicketPage(issue map[string]any, notes string) string {
	key := html.EscapeString(termsafe.Clean(str(issue, "key")))
	if notes == "" {
		today := time.Now().Format("2006-01-02")
		notes = fmt.Sprintf(
			`<p><time datetime="%s" /></p>`+
				`<ac:adf-extension><ac:adf-node type="panel"><ac:adf-attribute key="panelType">note</ac:adf-attribute><ac:adf-content><p> </p></ac:adf-content></ac:adf-node></ac:adf-extension>`,
			today,
		)
	}
	return fmt.Sprintf(
		`<!-- jira-thing:details:start --><h2>Jira Details</h2>`+
			`<ac:structured-macro ac:name="jira" ac:schema-version="1">`+
			`<ac:parameter ac:name="key">%s</ac:parameter>`+
			`</ac:structured-macro>`+
			`<!-- jira-thing:details:end -->`+
			`<h2>Notes</h2>%s%s%s`,
		key, notesStart, notes, notesEnd,
	)
}

// renderHangerPage builds a Confluence storage-format page listing tickets and any manually
// created child pages as links.
func renderHangerPage(issues []map[string]any, manualPages []api.ConfluencePageWithBody) string {
	if len(issues) == 0 && len(manualPages) == 0 {
		return fmt.Sprintf("<p>No open TOIL tickets as of %s.</p>", time.Now().Format("2006-01-02"))
	}
	var sb strings.Builder
	sb.WriteString("<ul>")
	for _, issue := range issues {
		key := html.EscapeString(termsafe.Clean(str(issue, "key")))
		f, _ := issue["fields"].(map[string]any)
		summary := ""
		if f != nil {
			summary = html.EscapeString(termsafe.Clean(str(f, "summary")))
		}
		fmt.Fprintf(&sb,
			`<li><ac:link><ri:page ri:content-title=%q/><ac:plain-text-link-body><![CDATA[%s - %s]]></ac:plain-text-link-body></ac:link></li>`,
			key, key, summary)
	}
	for _, p := range manualPages {
		title := html.EscapeString(p.Title)
		fmt.Fprintf(&sb,
			`<li><ac:link><ri:page ri:content-title=%q/><ac:plain-text-link-body><![CDATA[%s]]></ac:plain-text-link-body></ac:link></li>`,
			title, title)
	}
	sb.WriteString("</ul>")
	return sb.String()
}
