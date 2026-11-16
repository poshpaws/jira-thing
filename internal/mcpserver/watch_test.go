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

// watchServer fakes myself, user search and watchers, counting mutations.
func watchServer(t *testing.T, watchers []map[string]any, users []map[string]any, mutations *int) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/myself"):
			json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me", "displayName": "Me"})
		case strings.HasSuffix(r.URL.Path, "/user/search"):
			json.NewEncoder(w).Encode(users)
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"watchers": watchers})
		default:
			*mutations++
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL}
}

func TestHandleWatch(t *testing.T) {
	two := []map[string]any{person("a1", "Ada One"), person("a2", "Ada Two")}
	tests := []struct {
		name          string
		watch         bool
		args          map[string]any
		want          string
		wantMutations int
	}{
		{"watch me", true, map[string]any{"ticket_key": "K-1"}, "Me (acc-me) now watching K-1", 1},
		{"unwatch dry run", false, map[string]any{"ticket_key": "K-1", "dry_run": true}, "Dry run: Me (acc-me) would stop watching K-1", 0},
		{"ambiguous", true, map[string]any{"ticket_key": "K-1", "user": "ada"}, "Retry with one of the account IDs", 0},
		{"missing key", true, map[string]any{}, "ticket_key", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			conn := watchServer(t, nil, two, &mutations)
			result, _ := handleWatch(conn, tt.watch)(context.Background(), newCallToolRequest(tt.args))
			if text := resultText(t, result); !strings.Contains(text, tt.want) || mutations != tt.wantMutations {
				t.Errorf("mutations=%d text=%s", mutations, text)
			}
		})
	}
}

func TestHandleListWatchers(t *testing.T) {
	mutations := 0
	conn := watchServer(t, []map[string]any{{"accountId": "a1", "displayName": "Ada\n- Forged"}}, nil, &mutations)
	if text := callAgile(t, handleListWatchers, conn, map[string]any{"ticket_key": "K-1"}); !strings.Contains(text, "- Ada - Forged (a1)") {
		t.Errorf("text = %s", text)
	}
	empty := watchServer(t, nil, nil, &mutations)
	if text := callAgile(t, handleListWatchers, empty, map[string]any{"ticket_key": "K-1"}); !strings.Contains(text, "Nobody is watching K-1") {
		t.Errorf("text = %s", text)
	}
}
