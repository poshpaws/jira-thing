package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/config"
)

func TestRunEpicList_UsesConfigProject(t *testing.T) {
	var capturedJQL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		capturedJQL, _ = body["jql"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{
				map[string]any{"key": "PROJ-1", "fields": map[string]any{
					"summary": "Epic one", "updated": "2026-05-20T10:00:00.000Z",
					"status": map[string]any{"name": "Open"}, "priority": map[string]any{"name": "Medium"},
				}},
			},
			"total": 1, "maxResults": 100,
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfig("TOIL_LABEL", "SEC_TEAM")()

	out := captureStdout(func() { runEpicList([]string{}) })
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("expected PROJ-1 in output, got: %s", out)
	}
	if !strings.Contains(capturedJQL, `project = "PROJ"`) {
		t.Errorf("expected JQL to use config project, got: %s", capturedJQL)
	}
}

func TestRunEpicList_ProjectFlagOverridesConfig(t *testing.T) {
	var capturedJQL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		capturedJQL, _ = body["jql"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total": 0})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	captureStdout(func() { runEpicList([]string{"-p", "OTHER"}) })
	if !strings.Contains(capturedJQL, `project = "OTHER"`) {
		t.Errorf("expected JQL to use -p override, got: %s", capturedJQL)
	}
}

func TestRunEpicList_NoProjectConfigured(t *testing.T) {
	old := config.ConfigPath
	config.ConfigPath = func() string { return "/nonexistent/path.json" }
	defer func() { config.ConfigPath = old }()

	didExit := captureExit(func() {
		captureStderr(func() { runEpicList([]string{}) })
	})
	if !didExit {
		t.Error("expected fatal() when no project is configured")
	}
}

func TestRunEpicDescribe_ListsIssuesUnderEpic(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{
				map[string]any{"key": "PROJ-2", "fields": map[string]any{
					"summary": "Story under epic", "updated": "2026-05-20T10:00:00.000Z",
					"status": map[string]any{"name": "Open"}, "priority": map[string]any{"name": "Medium"},
				}},
			},
			"total": 1, "maxResults": 100,
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runEpicDescribe([]string{"PROJ-1"}) })
	if !strings.Contains(out, "PROJ-2") {
		t.Errorf("expected PROJ-2 in output, got: %s", out)
	}
}

func TestRunEpicDescribe_NoArgs(t *testing.T) {
	didExit := captureExit(func() { runEpicDescribe([]string{}) })
	if !didExit {
		t.Error("expected fatal() with no epic key")
	}
}
