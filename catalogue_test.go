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

// catalogueHandler serves boards, projects and versions.
func catalogueHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var values []map[string]any
	switch {
	case strings.HasSuffix(r.URL.Path, "/version"):
		values = []map[string]any{
			{"name": "v2", "releaseDate": "2026-12-01"},
			{"name": "v1", "released": true, "archived": true},
		}
	case strings.HasSuffix(r.URL.Path, "/project/search"):
		values = []map[string]any{{"key": "PROJ", "name": "Project"}}
	default:
		values = []map[string]any{{"id": 1, "name": "Team", "type": "simple"}}
	}
	json.NewEncoder(w).Encode(map[string]any{"isLast": true, "values": values})
}

// startCatalogue serves the fake API with cfgJSON as the config file.
func startCatalogue(t *testing.T, cfgJSON string) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(catalogueHandler))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	path := filepath.Join(t.TempDir(), "jira-thing.json")
	if err := os.WriteFile(path, []byte(cfgJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	t.Cleanup(func() { config.ConfigPath = old })
}

func TestCatalogueLists(t *testing.T) {
	tests := []struct {
		name string
		run  func([]string)
		args []string
		want []string
		not  string
	}{
		{"boards", runBoard, []string{"list"}, []string{"[1]", "Team", "simple"}, ""},
		{"boards json", runBoard, []string{"list", "-p", "PROJ", "-json"}, []string{`"name":"Team"`}, ""},
		{"projects", runProject, []string{"list"}, []string{"PROJ", "Project"}, ""},
		{"projects json", runProject, []string{"list", "-json"}, []string{`"key":"PROJ"`}, ""},
		{"releases hide archived", runRelease, []string{"list"}, []string{"v2", "unreleased", "2026-12-01"}, "v1"},
		{"releases all", runRelease, []string{"list", "-all"}, []string{"v1", "archived"}, ""},
		{"releases json", runRelease, []string{"list", "-p", "PROJ", "-unreleased", "-json"}, []string{`"name":"v2"`}, "v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startCatalogue(t, `{"project":"PROJ"}`)
			out := captureStdout(func() { tt.run(tt.args) })
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in %s", w, out)
				}
			}
			if tt.not != "" && strings.Contains(out, tt.not) {
				t.Errorf("unexpected %q in %s", tt.not, out)
			}
		})
	}
}

func TestRunReleaseList_NoProject(t *testing.T) {
	startCatalogue(t, `{}`)
	errOut := captureStderr(func() {
		if !captureExit(func() { runRelease([]string{"list"}) }) {
			t.Error("expected exit")
		}
	})
	if !strings.Contains(errOut, "project key required") {
		t.Errorf("stderr = %s", errOut)
	}
}

func TestCatalogue_Usage(t *testing.T) {
	for _, run := range []func([]string){runBoard, runProject, runRelease} {
		for _, args := range [][]string{nil, {"bogus"}, {"list", "extra"}} {
			captureStderr(func() {
				if !captureExit(func() { run(args) }) {
					t.Errorf("args %v: expected exit", args)
				}
			})
		}
	}
}

func TestCatalogueMenus(t *testing.T) {
	startCatalogue(t, `{"project":"PROJ"}`)
	conn := mustConnect()
	defer pipeStdinLines(t, "")()
	out := captureStdout(func() {
		menuListBoards(conn)
		menuListProjects(conn)
		menuListVersions(conn)
	})
	for _, w := range []string{"Team", "Project", "v2"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in %s", w, out)
		}
	}
}
