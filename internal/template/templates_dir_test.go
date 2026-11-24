package template

import (
	"os"
	"path/filepath"
	"testing"
)

// withCandidates points the fallback search path at paths for one test.
func withCandidates(t *testing.T, paths ...string) {
	t.Helper()
	old := CandidatePathsFunc
	CandidatePathsFunc = func() []string { return paths }
	t.Cleanup(func() { CandidatePathsFunc = old })
}

func writeTemplate(t *testing.T, dir, summary string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, defaultTemplateFile), []byte(`{"summary":"`+summary+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFromDir(t *testing.T) {
	fallback, configured, empty := t.TempDir(), t.TempDir(), t.TempDir()
	writeTemplate(t, fallback, "fallback")
	writeTemplate(t, configured, "configured")
	withCandidates(t, filepath.Join(fallback, defaultTemplateFile))
	fallbackFile := filepath.Join(fallback, defaultTemplateFile)
	tests := []struct {
		name, path, dir, want, wantFrom string
	}{
		{"templates dir searched first", "", configured, "configured", filepath.Join(configured, defaultTemplateFile)},
		{"empty templates dir falls back", "", empty, "fallback", fallbackFile},
		{"no templates dir", "", "", "fallback", fallbackFile},
		{"explicit path wins", fallbackFile, configured, "fallback", fallbackFile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, from, err := LoadFromDir(tt.path, tt.dir)
			if err != nil || tmpl["summary"] != tt.want || from != tt.wantFrom {
				t.Errorf("tmpl=%v from=%q err=%v", tmpl, from, err)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := ExpandHome("~/tpl"); got != filepath.Join(home, "tpl") {
		t.Errorf("got %q", got)
	}
	if got := ExpandHome("/abs/tpl"); got != "/abs/tpl" {
		t.Errorf("got %q", got)
	}
}
