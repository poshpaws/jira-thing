package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"jira-thing/internal/config"
	"jira-thing/internal/template"
)

// SourceBuiltIn is TemplateInfo.Source for a template derived from config
// rather than read from a file.
const SourceBuiltIn = "built-in"

// TemplateInfo names a template create can use by name, and where it comes
// from: a file path, or SourceBuiltIn.
type TemplateInfo struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// UnknownTemplateError means no file or built-in has the name; Available
// lists the ones that do exist.
type UnknownTemplateError struct {
	Name      string
	Dir       string
	Available []string
}

func (e *UnknownTemplateError) Error() string {
	return fmt.Sprintf("no template named %q in %s and no built-in of that name. Available: %s",
		e.Name, e.Dir, strings.Join(e.Available, ", "))
}

// builtInTemplates are the templates every setup has, derived from cfg:
// "standard" is a Task in the configured project, and "toil" (only when
// toil_marker is set) adds the labels toil-check looks for.
func builtInTemplates(cfg config.Config) map[string]map[string]any {
	standard := func() map[string]any {
		tmpl := map[string]any{"issuetype": DefaultIssueType}
		setIfGiven(tmpl, "project", cfg.Project)
		return tmpl
	}
	builtIns := map[string]map[string]any{"standard": standard()}
	if cfg.ToilMarker != "" {
		toil := standard()
		toil["labels"] = toilLabels(cfg)
		builtIns["toil"] = toil
	}
	return builtIns
}

// toilLabels are the labels that make a ticket toil: the marker, plus the
// team label unless the team is matched by the Team field instead.
func toilLabels(cfg config.Config) []string {
	if cfg.UseTeamField || cfg.ToilTeam == "" {
		return []string{cfg.ToilMarker}
	}
	return []string{cfg.ToilMarker, cfg.ToilTeam}
}

// ListTemplates returns the named templates, sorted: the files in the
// templates directory, and any built-in without a file of the same name.
func ListTemplates(cfg config.Config) ([]TemplateInfo, error) {
	dir := template.NamedDir(cfg.TemplatesDir)
	names, err := template.ListNamed(dir)
	if err != nil {
		return nil, err
	}
	byName := map[string]string{}
	for name := range builtInTemplates(cfg) {
		byName[name] = SourceBuiltIn
	}
	for _, name := range names {
		byName[name] = filepath.Join(dir, name+".json")
	}
	infos := make([]TemplateInfo, 0, len(byName))
	for name, source := range byName {
		infos = append(infos, TemplateInfo{Name: name, Source: source})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// LoadTemplate returns template name: the file in the templates directory if
// there is one, else the built-in of that name.
func LoadTemplate(cfg config.Config, name string) (map[string]any, error) {
	dir := template.NamedDir(cfg.TemplatesDir)
	tmpl, err := template.LoadNamed(dir, name)
	if err == nil || !os.IsNotExist(err) {
		return tmpl, err
	}
	if builtIn, ok := builtInTemplates(cfg)[name]; ok {
		return builtIn, nil
	}
	return nil, unknownTemplate(cfg, dir, name)
}

// unknownTemplate builds the error for a missing name, listing what exists.
func unknownTemplate(cfg config.Config, dir, name string) error {
	infos, _ := ListTemplates(cfg)
	available := make([]string, len(infos))
	for i, info := range infos {
		available[i] = info.Name
	}
	return &UnknownTemplateError{Name: name, Dir: dir, Available: available}
}
