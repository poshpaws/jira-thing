package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/config"
	"jira-thing/internal/template"
)

// withCreateConfig points config at cfgJSON and starts a server that fails
// on any request, for dry runs.
func withCreateConfig(t *testing.T, cfgJSON string) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request in dry-run mode: %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	path := filepath.Join(t.TempDir(), "jira-thing.json")
	os.WriteFile(path, []byte(cfgJSON), 0o600)
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	t.Cleanup(func() { config.ConfigPath = old })
}

func TestRunCreate_ConfigDefaultsFillUnsetFields(t *testing.T) {
	withCreateConfig(t, `{"default_priority":"High","default_components":["Backend"],"default_labels":["cfg"]}`)
	tmpl := filepath.Join(t.TempDir(), "tpl.json")
	os.WriteFile(tmpl, []byte(`{"project":"PROJ","summary":"S","labels":["tpl"]}`), 0o600)

	out := captureStdout(func() { runCreate([]string{"-t", tmpl, "-y", "-dry-run", "-components", "Frontend"}) })
	for _, want := range []string{`"name": "High"`, `"name": "Frontend"`, `"tpl"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in payload:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{`"Backend"`, `"cfg"`} {
		if strings.Contains(out, unwanted) {
			t.Errorf("default %s should be overridden:\n%s", unwanted, out)
		}
	}
}

func TestRunCreate_TemplatesDirSearchedFirst(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ticket_template.json"), []byte(`{"project":"PROJ","summary":"from templates_dir"}`), 0o600)
	withCreateConfig(t, `{"templates_dir":"`+dir+`"}`)
	old := template.CandidatePathsFunc
	template.CandidatePathsFunc = func() []string { return nil }
	t.Cleanup(func() { template.CandidatePathsFunc = old })

	out := captureStdout(func() { runCreate([]string{"-y", "-dry-run"}) })
	if !strings.Contains(out, "from templates_dir") {
		t.Errorf("expected the templates_dir template, got:\n%s", out)
	}
}
