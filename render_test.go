package main

import (
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func TestBuildDescribeMarkdown_FullIssue(t *testing.T) {
	issue := map[string]any{
		"key": "PROJ-1",
		"fields": map[string]any{
			"summary":  "Fix the thing",
			"status":   map[string]any{"name": "In Progress"},
			"priority": map[string]any{"name": "High"},
			"assignee": map[string]any{"displayName": "Alice"},
			"reporter": map[string]any{"displayName": "Bob"},
			"created":  "2026-01-01T10:00:00.000+0000",
			"updated":  "2026-01-02T10:00:00.000+0000",
			"description": map[string]any{
				"type": "doc",
				"content": []any{
					map[string]any{
						"type":    "paragraph",
						"content": []any{map[string]any{"type": "text", "text": "does the thing"}},
					},
				},
			},
		},
	}
	got := buildDescribeMarkdown(issue)
	want := "# PROJ-1: Fix the thing\n\n" +
		"- **Status:** In Progress\n" +
		"- **Priority:** High\n" +
		"- **Assignee:** Alice\n" +
		"- **Reporter:** Bob\n" +
		"- **Created:** 2026-01-01\n" +
		"- **Updated:** 2026-01-02\n\n" +
		"## Description\n\n" +
		"does the thing\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildDescribeMarkdown_MissingFields(t *testing.T) {
	issue := map[string]any{"key": "PROJ-2"}
	got := buildDescribeMarkdown(issue)
	want := "# PROJ-2: \n\n" +
		"- **Status:** \n" +
		"- **Priority:** \n" +
		"- **Assignee:** Unassigned\n" +
		"- **Reporter:** Unknown\n" +
		"- **Created:** \n" +
		"- **Updated:** \n\n" +
		"## Description\n\n" +
		"_No description._\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// captureStdout's pipe isn't a terminal, so these exercise the auto-plain path
// that "describe KEY | cat" relies on: no ANSI escape codes in the output.

func TestRenderDescribe_PipedOutputHasNoEscapeCodes(t *testing.T) {
	issue := map[string]any{
		"key":    "PROJ-1",
		"fields": map[string]any{"summary": "Fix the thing"},
	}
	out := captureStdout(func() { renderDescribe(issue, false, false) })
	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("expected no ANSI escape codes in piped output, got: %q", out)
	}
	if !strings.Contains(out, "PROJ-1") || !strings.Contains(out, "Fix the thing") {
		t.Errorf("expected plain markdown content, got: %q", out)
	}
}

func TestRenderDescribe_RawPrintsIssueJSON(t *testing.T) {
	issue := map[string]any{
		"key": "PROJ-1",
		"fields": map[string]any{
			"description": map[string]any{"type": "doc", "version": 1.0},
		},
	}
	out := captureStdout(func() { renderDescribe(issue, false, true) })
	if !strings.Contains(out, `"type": "doc"`) {
		t.Errorf("expected raw ADF description in JSON output, got: %q", out)
	}
	if strings.Contains(out, "## Description") {
		t.Errorf("expected raw mode to skip markdown rendering, got: %q", out)
	}
}

func TestRenderLastComment_PipedOutputHasNoEscapeCodes(t *testing.T) {
	comment := api.Comment{
		Author:  map[string]any{"displayName": "Alice"},
		Created: "2026-01-01T10:00:00.000+0000",
		Body: map[string]any{
			"type": "doc",
			"content": []any{
				map[string]any{
					"type":    "paragraph",
					"content": []any{map[string]any{"type": "text", "text": "hello"}},
				},
			},
		},
	}
	out := captureStdout(func() { renderLastComment(comment, false, false) })
	if strings.ContainsRune(out, '\x1b') {
		t.Errorf("expected no ANSI escape codes in piped output, got: %q", out)
	}
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "hello") {
		t.Errorf("expected plain markdown content, got: %q", out)
	}
}

func TestRenderLastComment_RawPrintsBodyJSON(t *testing.T) {
	comment := api.Comment{
		Author:  map[string]any{"displayName": "Alice"},
		Created: "2026-01-01T10:00:00.000+0000",
		Body:    map[string]any{"type": "doc", "version": 1.0},
	}
	out := captureStdout(func() { renderLastComment(comment, false, true) })
	if !strings.Contains(out, `"type": "doc"`) {
		t.Errorf("expected raw ADF body in JSON output, got: %q", out)
	}
}

func TestBuildDescribeMarkdown_ListsLinkedTickets(t *testing.T) {
	issue := map[string]any{
		"key": "PROJ-1",
		"fields": map[string]any{
			"issuelinks": []any{
				map[string]any{
					"type": map[string]any{"inward": "is blocked by", "outward": "blocks"},
					"inwardIssue": map[string]any{
						"key":    "PROJ-9",
						"fields": map[string]any{"summary": "Upstream", "status": map[string]any{"name": "Done"}},
					},
				},
			},
		},
	}
	got := buildDescribeMarkdown(issue)
	want := "## Linked Tickets\n\n- is blocked by PROJ-9: Upstream (Done)\n\n## Description"
	if !strings.Contains(got, want) {
		t.Errorf("missing linked tickets section before description:\n%s", got)
	}
}

func TestRenderDescribe_StripsEscapesFromJiraText(t *testing.T) {
	issue := map[string]any{
		"key": "PROJ-1",
		"fields": map[string]any{
			"summary":  "Fix\x1b]0;pwned\x07 it‮",
			"status":   map[string]any{"name": "Open\x1b[2J"},
			"assignee": map[string]any{"displayName": "Eve\x1b[8m"},
		},
	}
	out := captureStdout(func() { renderDescribe(issue, true, false) })
	for _, bad := range []rune{'\x1b', '\x07', '‮'} {
		if strings.ContainsRune(out, bad) {
			t.Errorf("control character %q survived: %q", bad, out)
		}
	}
	if !strings.Contains(out, "Fix]0;pwned it") {
		t.Errorf("expected visible text kept, got: %q", out)
	}
}

func TestIssuesToTickets_StripsEscapes(t *testing.T) {
	issues := []map[string]any{{"key": "PROJ-1", "fields": map[string]any{"summary": "a\x1b[31mb"}}}
	if got := issuesToTickets(issues)[0].Summary; got != "a[31mb" {
		t.Errorf("got %q", got)
	}
}
