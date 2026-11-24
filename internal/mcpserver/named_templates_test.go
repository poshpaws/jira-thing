package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/config"
)

func TestHandleListTemplates(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bug.json"), []byte(`{}`), 0o600)
	withConfig(t, config.Config{TemplatesDir: dir})
	result, err := handleListTemplates()(context.Background(), newCallToolRequest(nil))
	text := resultText(t, result)
	if err != nil || !strings.Contains(text, "bug") || !strings.Contains(text, "standard (built-in)") {
		t.Errorf("text = %s", text)
	}
}

func TestCreateRequest_TemplateIsBase(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bug.json"), []byte(`{"issuetype":{"name":"Bug"},"labels":["x"]}`), 0o600)
	withConfig(t, config.Config{TemplatesDir: dir})
	got, err := createRequest(newCallToolRequest(map[string]any{"project": "PROJ", "summary": "S", "template": "bug"}))
	if err != nil || got.Fields["labels"] == nil || got.Project != "PROJ" {
		t.Errorf("req=%+v err=%v", got, err)
	}
	if _, err := createRequest(newCallToolRequest(map[string]any{"project": "PROJ", "summary": "S", "template": "../etc"})); err == nil {
		t.Error("expected an unsafe template name to be rejected")
	}
}
