package ops

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"jira-thing/internal/config"
)

// namedDir writes the given templates to a fresh directory.
func namedDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o600)
	}
	return dir
}

func TestListTemplates_UserFilesOverrideBuiltIns(t *testing.T) {
	dir := namedDir(t, map[string]string{"standard": `{}`, "bug": `{}`})
	got, err := ListTemplates(config.Config{TemplatesDir: dir, ToilMarker: "TOIL"})
	if err != nil {
		t.Fatal(err)
	}
	var names, sources []string
	for _, info := range got {
		names = append(names, info.Name)
		sources = append(sources, info.Source)
	}
	if !slices.Equal(names, []string{"bug", "standard", "toil"}) {
		t.Errorf("names = %v", names)
	}
	if sources[1] != filepath.Join(dir, "standard.json") || sources[2] != SourceBuiltIn {
		t.Errorf("sources = %v", sources)
	}
}

func TestListTemplates_ToilNeedsMarker(t *testing.T) {
	got, _ := ListTemplates(config.Config{TemplatesDir: t.TempDir()})
	if len(got) != 1 || got[0].Name != "standard" {
		t.Errorf("got %+v", got)
	}
}

func TestLoadTemplate(t *testing.T) {
	dir := namedDir(t, map[string]string{"standard": `{"summary":"mine"}`})
	cfg := config.Config{TemplatesDir: dir, Project: "PROJ", ToilMarker: "TOIL", ToilTeam: "Ops"}

	if tmpl, err := LoadTemplate(cfg, "standard"); err != nil || tmpl["summary"] != "mine" {
		t.Errorf("user standard: %v err=%v", tmpl, err)
	}
	toil, err := LoadTemplate(cfg, "toil")
	labels, _ := toil["labels"].([]string)
	if err != nil || toil["project"] != "PROJ" || !slices.Equal(labels, []string{"TOIL", "Ops"}) {
		t.Errorf("toil: %v err=%v", toil, err)
	}
	var unknown *UnknownTemplateError
	if _, err := LoadTemplate(cfg, "nope"); !errors.As(err, &unknown) || !slices.Contains(unknown.Available, "toil") {
		t.Errorf("unknown: err=%v", err)
	}
	if _, err := LoadTemplate(cfg, "../x"); err == nil {
		t.Error("expected an unsafe name to be rejected")
	}
}

func TestLoadTemplate_ToilWithTeamFieldOmitsTeamLabel(t *testing.T) {
	cfg := config.Config{TemplatesDir: t.TempDir(), ToilMarker: "TOIL", ToilTeam: "Ops", UseTeamField: true}
	toil, _ := LoadTemplate(cfg, "toil")
	if labels, _ := toil["labels"].([]string); !slices.Equal(labels, []string{"TOIL"}) {
		t.Errorf("labels = %v", toil["labels"])
	}
}
