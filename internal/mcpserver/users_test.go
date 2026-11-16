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

// usersServer answers user search with users and counts assignee PUTs.
func usersServer(t *testing.T, users []map[string]any, assigns *int) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/assignee") {
			*assigns++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(users)
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL}
}

func person(id, name string) map[string]any {
	return map[string]any{"accountId": id, "displayName": name, "active": true, "accountType": "atlassian"}
}

func TestHandleSearchUsers(t *testing.T) {
	assigns := 0
	conn := usersServer(t, []map[string]any{person("a1", "Ada")}, &assigns)
	result, _ := handleSearchUsers(conn)(context.Background(), newCallToolRequest(map[string]any{"query": "ad"}))
	if text := resultText(t, result); !strings.Contains(text, "- Ada (a1)") {
		t.Errorf("text = %s", text)
	}
	empty := usersServer(t, nil, &assigns)
	result, _ = handleSearchUsers(empty)(context.Background(), newCallToolRequest(map[string]any{"query": "zed"}))
	if text := resultText(t, result); !strings.Contains(text, "No active users") {
		t.Errorf("text = %s", text)
	}
}

func TestHandleAssignTicket(t *testing.T) {
	tests := []struct {
		name        string
		users       []map[string]any
		args        map[string]any
		wantText    string
		wantAssigns int
	}{
		{"by name", []map[string]any{person("a1", "Ada")}, map[string]any{"ticket_key": "K-1", "assignee": "ada"}, "Assigned K-1 to Ada (a1)", 1},
		{"dry run", []map[string]any{person("a1", "Ada")}, map[string]any{"ticket_key": "K-1", "assignee": "ada", "dry_run": true}, "Dry run: would assign K-1 to Ada", 0},
		{"ambiguous", []map[string]any{person("a1", "Ada One"), person("a2", "Ada Two")}, map[string]any{"ticket_key": "K-1", "assignee": "ada"}, "Retry with one of the account IDs", 0},
		{"missing assignee", nil, map[string]any{"ticket_key": "K-1"}, "assignee", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assigns := 0
			conn := usersServer(t, tt.users, &assigns)
			result, _ := handleAssignTicket(conn)(context.Background(), newCallToolRequest(tt.args))
			if text := resultText(t, result); !strings.Contains(text, tt.wantText) || assigns != tt.wantAssigns {
				t.Errorf("assigns=%d text=%s", assigns, text)
			}
		})
	}
}
