package template_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"jira-client/internal/template"
)

var sampleIssue = map[string]any{
	"key": "PROJ-123",
	"fields": map[string]any{
		"project":     map[string]any{"key": "PROJ"},
		"issuetype":   map[string]any{"name": "Task"},
		"priority":    map[string]any{"name": "Medium"},
		"labels":      []any{"backend"},
		"components":  []any{map[string]any{"name": "api"}},
		"assignee":    map[string]any{"accountId": "abc123"},
		"summary":     "Original summary",
		"description": "Original description",
		"status":      map[string]any{"name": "Open"},
	},
}

func TestBuild_ExtractsTemplateFields(t *testing.T) {
	result := template.Build(sampleIssue)

	if proj, ok := result["project"].(map[string]any); !ok || proj["key"] != "PROJ" {
		t.Errorf("project = %v, want {key: PROJ}", result["project"])
	}
	if it, ok := result["issuetype"].(map[string]any); !ok || it["name"] != "Task" {
		t.Errorf("issuetype = %v", result["issuetype"])
	}
	if labels, ok := result["labels"].([]any); !ok || len(labels) != 1 {
		t.Errorf("labels = %v", result["labels"])
	}
}

func TestBuild_ExcludesNonTemplateFields(t *testing.T) {
	result := template.Build(sampleIssue)
	for _, field := range []string{"summary", "description", "status"} {
		if _, ok := result[field]; ok {
			t.Errorf("field %q should not be in template", field)
		}
	}
}

func TestBuild_SkipsNilFields(t *testing.T) {
	issue := map[string]any{
		"fields": map[string]any{
			"project":  map[string]any{"key": "X"},
			"priority": nil,
		},
	}
	result := template.Build(issue)
	if _, ok := result["priority"]; ok {
		t.Error("nil priority should be excluded from template")
	}
	if result["project"] == nil {
		t.Error("project should be included")
	}
}

func TestBuild_HandlesEmptyInput(t *testing.T) {
	if r := template.Build(map[string]any{}); len(r) != 0 {
		t.Errorf("expected empty result, got %v", r)
	}
	if r := template.Build(map[string]any{"fields": map[string]any{}}); len(r) != 0 {
		t.Errorf("expected empty result for empty fields, got %v", r)
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.json")
	tmpl := map[string]any{"project": map[string]any{"key": "PROJ"}, "issuetype": map[string]any{"name": "Bug"}}

	saved, err := template.Save(tmpl, path)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved != path {
		t.Errorf("saved path = %q, want %q", saved, path)
	}

	loaded, err := template.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	proj, _ := loaded["project"].(map[string]any)
	if proj["key"] != "PROJ" {
		t.Errorf("loaded project key = %v, want PROJ", proj["key"])
	}
}

func TestSave_CreatesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.json")
	tmpl := map[string]any{"labels": []any{"a", "b"}}

	if _, err := template.Save(tmpl, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, _ := os.ReadFile(path)
	var roundTripped map[string]any
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := template.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
