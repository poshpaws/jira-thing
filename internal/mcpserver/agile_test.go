package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"

	"github.com/mark3labs/mcp-go/server"
)

// agileServer fakes boards, sprints and sprint issues, counting sprint POSTs.
func agileServer(t *testing.T, boards []map[string]any, posts *int) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			*posts++
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/sprint"):
			json.NewEncoder(w).Encode(map[string]any{"isLast": true, "values": []map[string]any{
				{"id": 11, "name": "Sprint 11", "state": "active", "startDate": "2026-11-02T09:00:00.000Z", "endDate": "2026-11-16T09:00:00.000Z"},
			}})
		case strings.HasSuffix(r.URL.Path, "/issue"):
			json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{
				{"key": "PROJ-1", "fields": map[string]any{"summary": "First"}},
			}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"values": boards})
		}
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL}
}

// withConfig points config.Load at a file holding cfg.
func withConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jira-thing.json")
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	t.Cleanup(func() { config.ConfigPath = old })
}

var oneScrumBoard = []map[string]any{{"id": 4, "name": "Team board", "type": "scrum"}}

func callAgile(t *testing.T, handler func(api.JiraConnection) server.ToolHandlerFunc, conn api.JiraConnection, args map[string]any) string {
	t.Helper()
	result, _ := handler(conn)(context.Background(), newCallToolRequest(args))
	return resultText(t, result)
}

func TestHandleListSprints_UsesProjectBoard(t *testing.T) {
	withConfig(t, config.Config{Project: "PROJ"})
	posts := 0
	text := callAgile(t, handleListSprints, agileServer(t, oneScrumBoard, &posts), map[string]any{})
	if !strings.Contains(text, "[4] Team board") || !strings.Contains(text, "[11] Sprint 11 (active, 2026-11-02 → 2026-11-16)") {
		t.Errorf("text = %s", text)
	}
}

func TestHandleListSprints_SeveralBoards(t *testing.T) {
	withConfig(t, config.Config{Project: "PROJ"})
	posts := 0
	boards := []map[string]any{{"id": 1, "name": "A", "type": "scrum"}, {"id": 2, "name": "B", "type": "scrum"}}
	text := callAgile(t, handleListSprints, agileServer(t, boards, &posts), map[string]any{})
	if !strings.Contains(text, "Retry with board_id") || !strings.Contains(text, "[2] B") {
		t.Errorf("text = %s", text)
	}
}

func TestHandleListSprintIssues_DefaultsToCurrent(t *testing.T) {
	withConfig(t, config.Config{Board: 4})
	posts := 0
	text := callAgile(t, handleListSprintIssues, agileServer(t, nil, &posts), map[string]any{})
	if !strings.Contains(text, "Sprint [11] Sprint 11") || !strings.Contains(text, "PROJ-1") {
		t.Errorf("text = %s", text)
	}
}

func TestHandleAddToSprint(t *testing.T) {
	withConfig(t, config.Config{Board: 4})
	tests := []struct {
		name      string
		args      map[string]any
		wantText  string
		wantPosts int
	}{
		{"by id", map[string]any{"sprint_id": 42.0, "ticket_keys": "PROJ-1,PROJ-2"}, "Added 2 ticket(s) to sprint 42", 1},
		{"current dry run", map[string]any{"sprint": "current", "ticket_keys": "PROJ-1", "dry_run": true}, "Dry run: would add 1 ticket(s) to sprint [11]", 0},
		{"closed sprint", map[string]any{"sprint": "prev", "ticket_keys": "PROJ-1"}, "closed sprint", 0},
		{"no sprint", map[string]any{"ticket_keys": "PROJ-1"}, "sprint must be", 0},
		{"no keys", map[string]any{"sprint": "current"}, "ticket_keys", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			posts := 0
			text := callAgile(t, handleAddToSprint, agileServer(t, nil, &posts), tt.args)
			if !strings.Contains(text, tt.wantText) || posts != tt.wantPosts {
				t.Errorf("posts=%d text=%s", posts, text)
			}
		})
	}
}
