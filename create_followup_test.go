package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/config"
)

// followUpJira serves a create, a sprint add and an edit screen with one
// story point field, recording the sprint paths and PUT bodies.
type followUpJira struct {
	sprintPaths []string
	puts        []map[string]any
}

func (j *followUpJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(r.URL.Path, "/agile/1.0/sprint/"):
		j.sprintPaths = append(j.sprintPaths, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issue"):
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-500"})
	case strings.HasSuffix(r.URL.Path, "/editmeta"):
		json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{"customfield_10016": map[string]any{"name": "Story point estimate"}}})
	default:
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		j.puts = append(j.puts, body)
		w.WriteHeader(http.StatusNoContent)
	}
}

// startFollowUp starts j with an empty config and a template for PROJ,
// returning the template path.
func startFollowUp(t *testing.T, j http.Handler) string {
	t.Helper()
	srv := httptest.NewTLSServer(j)
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "jira-thing.json")
	os.WriteFile(cfgPath, []byte(`{}`), 0o600)
	old := config.ConfigPath
	config.ConfigPath = func() string { return cfgPath }
	t.Cleanup(func() { config.ConfigPath = old })
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"project":"PROJ","issuetype":{"name":"Task"},"summary":"S"}`), 0o600)
	return tmplPath
}

func TestRunCreate_SprintAndPointsAfterCreate(t *testing.T) {
	j := &followUpJira{}
	tmpl := startFollowUp(t, j)

	out := captureStdout(func() { runCreate([]string{"-t", tmpl, "-y", "-sprint", "42", "-points", "3"}) })
	if len(j.sprintPaths) != 1 || !strings.HasSuffix(j.sprintPaths[0], "/sprint/42/issue") {
		t.Errorf("sprint posts = %v", j.sprintPaths)
	}
	fields, _ := j.puts[0]["fields"].(map[string]any)
	if fields["customfield_10016"] != 3.0 {
		t.Errorf("PUT = %v", j.puts)
	}
	if !strings.Contains(out, "Sprint: 42") || !strings.Contains(out, "Story points: 3 (customfield_10016)") {
		t.Errorf("output = %s", out)
	}
}

func TestRunCreate_DryRunShowsSprintAndPointsSteps(t *testing.T) {
	tmpl := startFollowUp(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request in dry-run mode: %s %s", r.Method, r.URL.Path)
	}))

	out := captureStdout(func() { runCreate([]string{"-t", tmpl, "-y", "-sprint", "42", "-points", "2", "-dry-run"}) })
	if !strings.Contains(out, "add the new ticket to sprint 42") || !strings.Contains(out, "set story points to 2") {
		t.Errorf("output = %s", out)
	}
}

func TestRunCreate_BadPointsExitsBeforeCreate(t *testing.T) {
	tmpl := startFollowUp(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	captureStderr(func() {
		if !captureExit(func() { runCreate([]string{"-t", tmpl, "-y", "-points", "lots"}) }) {
			t.Error("expected exit on non-numeric -points")
		}
	})
}

func TestRunCreate_BatchDryRunShowsEntrySprintAndPoints(t *testing.T) {
	startFollowUp(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request in dry-run mode: %s %s", r.Method, r.URL.Path)
	}))
	batch := filepath.Join(t.TempDir(), "batch.json")
	os.WriteFile(batch, []byte(`[{"project":"PROJ","summary":"One","sprint":42,"points":2}]`), 0o600)

	out := captureStdout(func() { runCreate([]string{"-f", batch, "-dry-run"}) })
	if !strings.Contains(out, "add the new ticket to sprint 42") || !strings.Contains(out, "set story points to 2") || strings.Contains(out, `"points"`) {
		t.Errorf("output = %s", out)
	}
}
