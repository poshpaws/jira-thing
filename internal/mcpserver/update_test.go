package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// updateServer serves an edit screen and counts PUTs.
func updateServer(t *testing.T, puts *int) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/editmeta") {
			json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{
				"issuetype":         map[string]any{"allowedValues": []map[string]any{{"id": "3", "name": "Task"}}},
				"customfield_10308": map[string]any{"name": "Story Points"},
			}})
			return
		}
		*puts++
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL}
}

func TestHandleUpdateTicket(t *testing.T) {
	withConfig(t, config.Config{})
	tests := []struct {
		name     string
		args     map[string]any
		want     string
		wantPuts int
	}{
		{"type and points", map[string]any{"ticket_key": "K-1", "issue_type": "task", "story_points": 0.5}, "K-1 updated\nType: Task\nStory points: 0.5 (customfield_10308)", 1},
		{"zero points is set", map[string]any{"ticket_key": "K-1", "story_points": 0.0}, "Story points: 0 (customfield_10308)", 1},
		{"dry run", map[string]any{"ticket_key": "K-1", "issue_type": "Task", "dry_run": true}, `"id": "3"`, 0},
		{"unknown type", map[string]any{"ticket_key": "K-1", "issue_type": "Epic"}, "Types it can switch to: Task", 0},
		{"summary only", map[string]any{"ticket_key": "K-1", "summary": "S"}, "K-1 updated", 1},
		{"nothing", map[string]any{"ticket_key": "K-1"}, "no fields provided", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			puts := 0
			text := callAgile(t, handleUpdateTicket, updateServer(t, &puts), tt.args)
			if !strings.Contains(text, tt.want) || puts != tt.wantPuts {
				t.Errorf("puts=%d text=%s", puts, text)
			}
		})
	}
}
