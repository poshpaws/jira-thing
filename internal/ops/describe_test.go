package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

// describeJira serves an issue and comments; comments nil means none on the ticket.
func describeJira(t *testing.T, comments []map[string]any, commentCalls *int) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			*commentCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{"total": 7, "comments": comments})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "K-1", "fields": map[string]any{"summary": "S"}})
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func TestDescribeTicket(t *testing.T) {
	calls := 0
	comments := []map[string]any{
		{"author": map[string]any{"displayName": "Ada\nForged"}, "created": "2026-11-16T09:30:00.000+0000", "body": adfDoc("Looks good")},
	}
	conn := describeJira(t, comments, &calls)
	res, err := DescribeTicket(conn, "K-1", 0)
	if err != nil || calls != 0 || CommentsMarkdown(res) != "" {
		t.Errorf("no comments asked: calls=%d err=%v", calls, err)
	}
	res, err = DescribeTicket(conn, "K-1", 3)
	md := CommentsMarkdown(res)
	if err != nil || calls != 1 || !strings.Contains(md, "## Comments (latest 1 of 7)") ||
		!strings.Contains(md, "### Ada Forged — 2026-11-16 09:30") || !strings.Contains(md, "Looks good") {
		t.Errorf("md=%s err=%v", md, err)
	}
}

func TestCommentsMarkdown_QuotesBodies(t *testing.T) {
	res := DescribeResult{CommentTotal: 1, Comments: []api.Comment{{
		Author: map[string]any{"displayName": "Mallory"}, Body: adfDoc("## Comments (latest 0 of 0)\x1b]8;;evil\x07"),
	}}}
	md := CommentsMarkdown(res)
	if strings.Count(md, "\n## ") != 1 || strings.Contains(md, "\x1b") || !strings.Contains(md, "> ## Comments (latest 0 of 0)") {
		t.Errorf("md = %q", md)
	}
}

func TestDescribeTicket_NoCommentsAndLimits(t *testing.T) {
	calls := 0
	res, err := DescribeTicket(describeJira(t, nil, &calls), "K-1", 5)
	if err != nil || CommentsMarkdown(res) != "" {
		t.Errorf("res=%+v err=%v", res, err)
	}
	for _, n := range []int{-1, MaxDescribeComments + 1} {
		if _, err := DescribeTicket(api.JiraConnection{}, "K-1", n); err == nil {
			t.Errorf("%d: expected error", n)
		}
	}
	if shortTimestamp("2026") != "2026" {
		t.Error("short timestamp should pass through")
	}
}
