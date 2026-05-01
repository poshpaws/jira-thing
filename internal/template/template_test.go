package template_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/template"
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

func TestBuild_IncludesCustomFields(t *testing.T) {
	issue := map[string]any{
		"fields": map[string]any{
			"project":           map[string]any{"key": "PROJ"},
			"customfield_10001": map[string]any{"name": "My Team"},
			"customfield_10050": "some-value",
			"customfield_99999": nil,
		},
	}
	result := template.Build(issue)
	if result["customfield_10001"] == nil {
		t.Error("customfield_10001 (Team) should be included")
	}
	if result["customfield_10050"] == nil {
		t.Error("customfield_10050 should be included")
	}
	if _, ok := result["customfield_99999"]; ok {
		t.Error("nil custom field should be excluded")
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

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := template.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoad_ReadError(t *testing.T) {
	// Passing a directory path triggers a read error that is not os.IsNotExist.
	_, err := template.Load(t.TempDir())
	if err == nil {
		t.Fatal("expected error reading a directory as file, got nil")
	}
}

func TestLoad_DefaultFallbackErrorListsPaths(t *testing.T) {
	dir := t.TempDir()

	// Override candidate paths so only the empty temp dir is searched.
	original := template.CandidatePathsFunc
	template.CandidatePathsFunc = func() []string {
		return []string{filepath.Join(dir, "ticket_template.json")}
	}
	defer func() { template.CandidatePathsFunc = original }()

	_, loadErr := template.Load("")
	if loadErr == nil {
		t.Fatal("expected error when no template found in any location")
	}
	msg := loadErr.Error()
	if !strings.Contains(msg, "tried:") {
		t.Errorf("error should list tried paths, got: %s", msg)
	}
}

func TestLoad_FallbackCWD(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	tmpl := map[string]any{"project": map[string]any{"key": "CWD"}}
	if _, err := template.Save(tmpl, filepath.Join(dir, "ticket_template.json")); err != nil {
		t.Fatal(err)
	}

	loaded, err := template.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	proj, _ := loaded["project"].(map[string]any)
	if proj["key"] != "CWD" {
		t.Errorf("expected CWD project, got %v", proj)
	}
}

func TestSave_WriteError(t *testing.T) {
	_, err := template.Save(map[string]any{"key": "x"}, "/nonexistent/dir/file.json")
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory, got nil")
	}
}
