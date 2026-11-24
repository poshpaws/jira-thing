package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/template"
)

// withDefaultTemplate puts a default template carrying toil labels on the
// search path, as a real ~/.config/jira-thing/ticket_template.json can.
func withDefaultTemplate(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ticket_template.json")
	os.WriteFile(path, []byte(`{"project":"PROJ","issuetype":{"name":"Task"},"labels":["TOIL","FOBAR"]}`), 0o600)
	old := template.CandidatePathsFunc
	template.CandidatePathsFunc = func() []string { return []string{path} }
	t.Cleanup(func() { template.CandidatePathsFunc = old })
	return path
}

func TestRunCreate_NoTemplateIgnoresDefaultTemplate(t *testing.T) {
	withCreateConfig(t, `{}`)
	withDefaultTemplate(t)
	var out string
	errOut := captureStderr(func() {
		out = captureStdout(func() {
			runCreate([]string{"-no-template", "-project", "OTHER", "-summary", "S", "-y", "-dry-run"})
		})
	})
	if strings.Contains(out, "TOIL") || !strings.Contains(out, `"key": "OTHER"`) {
		t.Errorf("payload should hold only flag fields:\n%s", out)
	}
	if strings.Contains(errOut, "Using template") {
		t.Errorf("no template notice expected, got: %s", errOut)
	}
}

func TestRunCreate_SearchPathTemplateIsAnnounced(t *testing.T) {
	withCreateConfig(t, `{}`)
	path := withDefaultTemplate(t)
	errOut := captureStderr(func() {
		captureStdout(func() { runCreate([]string{"-project", "PROJ", "-summary", "S", "-y", "-dry-run"}) })
	})
	if !strings.Contains(errOut, "Using template: "+path) {
		t.Errorf("expected the template path on stderr, got: %q", errOut)
	}
}

func TestRunCreate_TemplateNoticeQuietForExplicitAndJSON(t *testing.T) {
	withCreateConfig(t, `{}`)
	path := withDefaultTemplate(t)
	tests := map[string][]string{
		"explicit -t": {"-t", path, "-summary", "S", "-y", "-dry-run"},
		"-T":          {"-T", "standard", "-project", "PROJ", "-summary", "S", "-y", "-dry-run"},
		"-json":       {"-project", "PROJ", "-summary", "S", "-y", "-dry-run", "-json"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			errOut := captureStderr(func() { captureStdout(func() { runCreate(args) }) })
			if strings.Contains(errOut, "Using template") {
				t.Errorf("unexpected notice: %q", errOut)
			}
		})
	}
}

func TestRunCreate_NoTemplateWithTemplateFlagsRefused(t *testing.T) {
	withCreateConfig(t, `{}`)
	for _, args := range [][]string{
		{"-no-template", "-t", "x.json", "-y"},
		{"-no-template", "-T", "standard", "-y"},
	} {
		errOut := captureStderr(func() {
			if !captureExit(func() { runCreate(args) }) {
				t.Errorf("%v: expected exit", args)
			}
		})
		if !strings.Contains(errOut, "-no-template") {
			t.Errorf("%v: stderr = %s", args, errOut)
		}
	}
}
