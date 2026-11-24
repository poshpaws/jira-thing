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

// createFollowUpServer serves a create, sprint adds and an edit screen with
// one story point field, recording sprint paths and PUT bodies.
func createFollowUpServer(t *testing.T, sprintPaths *[]string, puts *[]map[string]any) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/agile/1.0/sprint/"):
			*sprintPaths = append(*sprintPaths, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-7"})
		case strings.HasSuffix(r.URL.Path, "/editmeta"):
			json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{"customfield_10308": map[string]any{"name": "Story Points"}}})
		default:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			*puts = append(*puts, body)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL}
}

func TestHandleCreateTicket_SprintAndStoryPoints(t *testing.T) {
	withConfig(t, config.Config{})
	var sprintPaths []string
	var puts []map[string]any
	conn := createFollowUpServer(t, &sprintPaths, &puts)

	result, err := handleCreateTicket(conn)(context.Background(), newCallToolRequest(map[string]any{
		"project": "PROJ", "summary": "S", "sprint": "42", "story_points": 5.0,
	}))
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	text := resultText(t, result)
	if len(sprintPaths) != 1 || !strings.HasSuffix(sprintPaths[0], "/sprint/42/issue") {
		t.Errorf("sprint posts = %v", sprintPaths)
	}
	fields, _ := puts[0]["fields"].(map[string]any)
	if fields["customfield_10308"] != 5.0 {
		t.Errorf("PUT = %v", puts)
	}
	if !strings.Contains(text, "Added to sprint: 42") || !strings.Contains(text, "Story points: 5 (customfield_10308)") {
		t.Errorf("text = %s", text)
	}
}

func TestHandleCreateTicket_ClosedSprintIsToolError(t *testing.T) {
	withConfig(t, config.Config{})
	result, err := handleCreateTicket(api.JiraConnection{BaseURL: "https://example.invalid"})(context.Background(),
		newCallToolRequest(map[string]any{"project": "PROJ", "summary": "S", "sprint": "prev"}))
	if err != nil || !result.IsError {
		t.Errorf("expected a tool error, got %+v err=%v", result, err)
	}
}
