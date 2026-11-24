package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// describeServer serves PROJ-1 with nine comments, counting comment requests.
func describeServer(t *testing.T, commentCalls *int) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			*commentCalls++
			json.NewEncoder(w).Encode(map[string]any{"total": 9, "comments": []map[string]any{
				{"author": map[string]any{"displayName": "Ada"}, "created": "2026-11-16T09:30:00.000+0000"},
			}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1", "fields": map[string]any{"summary": "S"}})
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
}

func TestRunDescribe_LatestCommentByDefault(t *testing.T) {
	calls := 0
	describeServer(t, &calls)
	out := captureStdout(func() { runDescribe([]string{"PROJ-1", "-plain"}) })
	if calls != 1 || !strings.Contains(out, "Comments (latest 1 of 9)") || !strings.Contains(out, "Ada") {
		t.Errorf("calls=%d out=%s", calls, out)
	}
}

func TestRunDescribe_CommentsZeroSkipsThem(t *testing.T) {
	calls := 0
	describeServer(t, &calls)
	out := captureStdout(func() { runDescribe([]string{"PROJ-1", "-comments", "0", "-plain"}) })
	if calls != 0 || strings.Contains(out, "Comments") {
		t.Errorf("calls=%d out=%s", calls, out)
	}
}

func TestRunDescribe_RawKeepsIssueShapeByDefault(t *testing.T) {
	calls := 0
	describeServer(t, &calls)
	raw := captureStdout(func() { runDescribe([]string{"PROJ-1", "-raw"}) })
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil || got["key"] != "PROJ-1" || calls != 0 {
		t.Errorf("raw output must stay the bare issue: calls=%d err=%v raw=%s", calls, err, raw)
	}
}

func TestViewTicket_ShowsLatestComment(t *testing.T) {
	calls := 0
	describeServer(t, &calls)
	out := captureStdout(func() { viewTicket(mustConnect(), "PROJ-1") })
	if calls != 1 || !strings.Contains(out, "Ada") {
		t.Errorf("calls=%d out=%s", calls, out)
	}
}
