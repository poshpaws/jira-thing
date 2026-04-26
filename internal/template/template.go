package template

import (
	"encoding/json"
	"fmt"
	"os"
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
	fields, _ := issueData["fields"].(map[string]any)
	result := make(map[string]any)
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
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Load reads a template from a JSON file. Uses defaultTemplateFile if path is empty.
func Load(path string) (map[string]any, error) {
	if path == "" {
		path = defaultTemplateFile
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template not found: %s", path)
		}
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
