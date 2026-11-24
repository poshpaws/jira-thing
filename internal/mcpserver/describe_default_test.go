package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func TestHandleDescribeTicket_LatestCommentByDefault(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/comment") {
			json.NewEncoder(w).Encode(map[string]any{"total": 2, "comments": []map[string]any{
				{"author": map[string]any{"displayName": "Ada"}, "created": "2026-11-16T09:30:00.000+0000"},
			}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1", "fields": map[string]any{"summary": "S"}})
	}))
	t.Cleanup(srv.Close)
	result, err := handleDescribeTicket(api.JiraConnection{BaseURL: srv.URL})(context.Background(),
		newCallToolRequest(map[string]any{"ticket_key": "PROJ-1"}))
	if text := resultText(t, result); err != nil || !strings.Contains(text, "Comments (latest 1 of 2)") {
		t.Errorf("text = %s", text)
	}
}
