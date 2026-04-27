package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultTemplateFile = "ticket_template.json"

var templateFields = []string{
	"project",
	"issuetype",
	"priority",
	"labels",
	"components",
	"assignee",
}

// Build extracts the reusable fields from a raw Jira issue response.
func Build(issueData map[string]any) map[string]any {
	result := make(map[string]any)
	fields, ok := issueData["fields"].(map[string]any)
	if !ok {
		return result
	}
	for _, key := range templateFields {
		if v, ok := fields[key]; ok && v != nil {
			result[key] = v
		}
	}
	return result
}

// Save writes a template to a JSON file. Uses defaultTemplateFile if path is empty.
// Returns the path written to.
func Save(tmpl map[string]any, path string) (string, error) {
	if path == "" {
		path = defaultTemplateFile
	}
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// Load reads a template from a JSON file.
// When path is empty, tries cwd, binary directory, platform config dir, then ~/.config/jira-thing.
func Load(path string) (map[string]any, error) {
	if path != "" {
		return loadFile(path)
	}
	candidates := candidatePaths()
	for _, p := range candidates {
		tmpl, err := loadFile(p)
		if err == nil {
			return tmpl, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("template not found; tried: %s", strings.Join(candidates, ", "))
}

// candidatePaths returns ordered fallback locations for the default template file.
func candidatePaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, defaultTemplateFile))
	}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), defaultTemplateFile))
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(cfgDir, "jira-thing", defaultTemplateFile))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "jira-thing", defaultTemplateFile))
	}
	return paths
}

// loadFile reads and parses a JSON template from the given path.
func loadFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
