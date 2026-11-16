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

// startEditMeta serves an edit screen with Bug/Task and one story point
// field, records PUT bodies, and points config at cfgJSON.
func startEditMeta(t *testing.T, cfgJSON string) *[]map[string]any {
	t.Helper()
	puts := &[]map[string]any{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/editmeta") {
			json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{
				"issuetype":         map[string]any{"allowedValues": []map[string]any{{"id": "1", "name": "Bug"}, {"id": "3", "name": "Task"}}},
				"customfield_10016": map[string]any{"name": "Story point estimate"},
			}})
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*puts = append(*puts, body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	path := filepath.Join(t.TempDir(), "jira-thing.json")
	if err := os.WriteFile(path, []byte(cfgJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	t.Cleanup(func() { config.ConfigPath = old })
	return puts
}

func TestRunEdit_TypeAndPoints(t *testing.T) {
	puts := startEditMeta(t, `{}`)
	out := captureStdout(func() { runEdit([]string{"PROJ-1", "-type", "task", "-points", "0.5"}) })
	fields, _ := (*puts)[0]["fields"].(map[string]any)
	if fields["customfield_10016"] != 0.5 || !strings.Contains(out, "Type: Task") || !strings.Contains(out, "Story points: 0.5 (customfield_10016)") {
		t.Errorf("fields=%v out=%s", fields, out)
	}
}

func TestRunEdit_TypeAndPointsDryRun(t *testing.T) {
	puts := startEditMeta(t, `{}`)
	out := captureStdout(func() { runEdit([]string{"PROJ-1", "-type", "Task", "-points", "2", "-dry-run"}) })
	if len(*puts) != 0 || !strings.Contains(out, `"customfield_10016": 2`) || !strings.Contains(out, `"id": "3"`) {
		t.Errorf("puts=%v out=%s", *puts, out)
	}
}

func TestRunEdit_TypeAndPointsErrors(t *testing.T) {
	tests := []struct {
		name, cfg string
		args      []string
		want      string
	}{
		{"unknown type", `{}`, []string{"PROJ-1", "-type", "Epic"}, "Types it can switch to: Bug, Task"},
		{"bad points", `{}`, []string{"PROJ-1", "-points", "lots"}, "story points must be a number"},
		{"configured field missing", `{"story_points_field":"customfield_99999"}`, []string{"PROJ-1", "-points", "1"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			puts := startEditMeta(t, tt.cfg)
			var exited bool
			errOut := captureStderr(func() { captureStdout(func() { exited = captureExit(func() { runEdit(tt.args) }) }) })
			if tt.want == "" {
				// The configured field isn't on the screen, so the default one is used.
				if exited || len(*puts) != 1 {
					t.Errorf("exited=%v puts=%v stderr=%s", exited, *puts, errOut)
				}
				return
			}
			if !exited || !strings.Contains(errOut, tt.want) || len(*puts) != 0 {
				t.Errorf("exited=%v puts=%v stderr=%s", exited, *puts, errOut)
			}
		})
	}
}
