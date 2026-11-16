package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/server"
)

// lifecycleServer fakes issue fetch, create, delete, worklog and myself,
// counting mutating requests.
func lifecycleServer(t *testing.T, fields map[string]any, mutations *int) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			*mutations++
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/myself"):
			json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me", "displayName": "Me\nInjected", "emailAddress": "me@x.io"})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1", "fields": fields})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issue"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-9"})
		case strings.HasSuffix(r.URL.Path, "/worklog"):
			json.NewEncoder(w).Encode(map[string]any{"id": "1"})
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL}
}

func TestLifecycleTools(t *testing.T) {
	source := map[string]any{
		"project": map[string]any{"key": "PROJ"}, "issuetype": map[string]any{"name": "Task"},
		"summary": "Patch foo", "subtasks": []any{map[string]any{"key": "PROJ-2"}},
	}
	tests := []struct {
		name          string
		handler       func(api.JiraConnection) server.ToolHandlerFunc
		args          map[string]any
		want          string
		wantMutations int
	}{
		{"whoami one-line name", handleWhoami, map[string]any{}, "Display name: Me Injected", 0},
		{"delete refuses subtasks", handleDeleteTicket, map[string]any{"ticket_key": "PROJ-1"}, "subtasks", 0},
		{"delete dry run", handleDeleteTicket, map[string]any{"ticket_key": "PROJ-1", "cascade": true, "dry_run": true}, "Dry run: would delete PROJ-1: Patch foo\nand its subtasks: PROJ-2", 0},
		{"delete cascade", handleDeleteTicket, map[string]any{"ticket_key": "PROJ-1", "cascade": true}, "Deleted PROJ-1", 1},
		{"clone with replace", handleCloneTicket, map[string]any{"source_key": "PROJ-1", "replace": []any{"foo:bar"}, "labels": "a,b"}, "Cloned PROJ-1 -> PROJ-9", 1},
		{"clone dry run", handleCloneTicket, map[string]any{"source_key": "PROJ-1", "replace": []any{"foo:bar"}, "dry_run": true}, `"summary": "CLONE - Patch bar"`, 0},
		{"clone bad replace", handleCloneTicket, map[string]any{"source_key": "PROJ-1", "replace": []any{"oops"}}, "find:replace", 0},
		{"worklog", handleAddWorklog, map[string]any{"ticket_key": "PROJ-1", "time_spent": "2h"}, "Logged 2h on PROJ-1", 1},
		{"worklog dry run", handleAddWorklog, map[string]any{"ticket_key": "PROJ-1", "time_spent": "2h", "dry_run": true}, "Dry run: would log 2h", 0},
		{"worklog bad duration", handleAddWorklog, map[string]any{"ticket_key": "PROJ-1", "time_spent": "soon"}, "Jira's format", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			text := callAgile(t, tt.handler, lifecycleServer(t, source, &mutations), tt.args)
			if !strings.Contains(text, tt.want) || mutations != tt.wantMutations {
				t.Errorf("mutations=%d text=%s", mutations, text)
			}
		})
	}
}
