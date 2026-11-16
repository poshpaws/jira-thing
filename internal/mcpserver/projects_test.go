package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// catalogueServer serves boards, projects and versions.
func catalogueServer(t *testing.T) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var values []map[string]any
		switch {
		case strings.HasSuffix(r.URL.Path, "/version"):
			values = []map[string]any{
				{"name": "v2", "releaseDate": "2026-12-01", "overdue": true},
				{"name": "v1", "released": true, "archived": true},
			}
		case strings.HasSuffix(r.URL.Path, "/project/search"):
			values = []map[string]any{{"key": "PROJ", "name": "Project"}}
		default:
			values = []map[string]any{{"id": 1, "name": "Team", "type": "simple"}}
		}
		json.NewEncoder(w).Encode(map[string]any{"isLast": true, "values": values})
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL}
}

func TestHandleListVersions(t *testing.T) {
	withConfig(t, config.Config{Project: "CFG"})
	conn := catalogueServer(t)
	tests := []struct {
		name string
		args map[string]any
		want string
		not  string
	}{
		{"config project hides archived", map[string]any{}, "Releases in CFG:\n- v2 (overdue, 2026-12-01)", "v1"},
		{"include archived", map[string]any{"project_key": "PROJ", "include_archived": true}, "- v1 (archived)", ""},
		{"bad key", map[string]any{"project_key": "../x"}, "invalid project key", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := callAgile(t, handleListVersions, conn, tt.args)
			if !strings.Contains(text, tt.want) || tt.not != "" && strings.Contains(text, tt.not) {
				t.Errorf("text = %s", text)
			}
		})
	}
}

func TestHandleListVersions_NoProject(t *testing.T) {
	withConfig(t, config.Config{})
	if text := callAgile(t, handleListVersions, catalogueServer(t), map[string]any{}); !strings.Contains(text, "project key required") {
		t.Errorf("text = %s", text)
	}
}

func TestHandleListProjectsAndBoards(t *testing.T) {
	conn := catalogueServer(t)
	if text := callAgile(t, handleListProjects, conn, map[string]any{}); !strings.Contains(text, "- PROJ — Project") {
		t.Errorf("projects text = %s", text)
	}
	result, _ := handleListBoards(conn)(context.Background(), newCallToolRequest(map[string]any{}))
	if text := resultText(t, result); !strings.Contains(text, "[1] Team (simple)") {
		t.Errorf("boards text = %s", text)
	}
}
