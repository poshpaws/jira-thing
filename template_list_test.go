package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/ops"
)

// namedTemplatesConfig writes a bug template to a templates_dir and points
// config at it, with a toil marker so the toil built-in exists.
func namedTemplatesConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bug.json"), []byte(`{"project":"PROJ","issuetype":{"name":"Bug"},"summary":"from bug"}`), 0o600)
	withCreateConfig(t, `{"templates_dir":"`+dir+`","project":"PROJ","toil_marker":"TOIL"}`)
	return dir
}

func TestRunTemplateList(t *testing.T) {
	dir := namedTemplatesConfig(t)
	out := captureStdout(func() { runTemplate([]string{"list"}) })
	for _, want := range []string{"bug", filepath.Join(dir, "bug.json"), "standard", "toil", "built-in"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunTemplateList_JSON(t *testing.T) {
	namedTemplatesConfig(t)
	var got []ops.TemplateInfo
	out := captureStdout(func() { runTemplate([]string{"list", "-json"}) })
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 3 {
		t.Errorf("got %+v err=%v\n%s", got, err, out)
	}
}

func TestRunCreate_NamedTemplate(t *testing.T) {
	namedTemplatesConfig(t)
	out := captureStdout(func() { runCreate([]string{"-T", "bug", "-y", "-dry-run"}) })
	if !strings.Contains(out, "from bug") || !strings.Contains(out, `"Bug"`) {
		t.Errorf("expected the bug template, got:\n%s", out)
	}
	out = captureStdout(func() { runCreate([]string{"-T", "toil", "-summary", "S", "-y", "-dry-run"}) })
	if !strings.Contains(out, `"TOIL"`) {
		t.Errorf("expected the toil built-in's label, got:\n%s", out)
	}
}

func TestRunCreate_NamedTemplateErrors(t *testing.T) {
	namedTemplatesConfig(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown name lists the others", []string{"-T", "nope", "-y"}, "Available: bug, standard, toil"},
		{"unsafe name", []string{"-T", "../x", "-y"}, "template names use only"},
		{"both -t and -T", []string{"-t", "x.json", "-T", "bug", "-y"}, "not both"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStderr(func() {
				if !captureExit(func() { runCreate(tt.args) }) {
					t.Error("expected exit")
				}
			})
			if !strings.Contains(out, tt.want) {
				t.Errorf("stderr = %s", out)
			}
		})
	}
}
