package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultTemplateFile = "ticket_template.json"

// AssigneeSelf is a placeholder value in templates meaning "assign to the current user".
const AssigneeSelf = "__SELF__"

var templateFields = []string{
	"project",
	"issuetype",
	"priority",
	"labels",
	"components",
	"assignee",
}

// ExcludedFields contains Jira custom fields that do not support update
// and must never appear in templates or create payloads.
var ExcludedFields = []string{
	"customfield_10034",
	"customfield_10035",
	"customfield_10036",
	"customfield_10037",
	"customfield_10038",
	"customfield_10039",
	"customfield_10040",
	"customfield_10041",
	"customfield_10042",
	"customfield_10043",
	"customfield_10044",
}

// ResolveAssignee replaces the AssigneeSelf marker in a template with the given accountID.
// If the assignee is not the self marker, it is left unchanged.
func ResolveAssignee(tmpl map[string]any, accountID string) {
	if tmpl["assignee"] == AssigneeSelf {
		tmpl["assignee"] = map[string]any{"accountId": accountID}
	}
}

// StripExcludedFields removes all custom fields and other non-updatable fields from a template map.
func StripExcludedFields(tmpl map[string]any) map[string]any {
	for key := range tmpl {
		if strings.HasPrefix(key, "customfield_") || key == "rankBeforeIssue" || key == "rankAfterIssue" {
			delete(tmpl, key)
		}
	}
	return tmpl
}

// Build extracts the reusable fields from a raw Jira issue response.
// The assignee is stripped to just accountId and displayName.
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
	if assignee, ok := result["assignee"].(map[string]any); ok {
		result["assignee"] = map[string]any{
			"accountId":   assignee["accountId"],
			"displayName": assignee["displayName"],
		}
	}
	return result
}

// Save writes a template to a JSON file. Uses defaultTemplateFile if path is empty.
// If path is an existing directory, writes defaultTemplateFile inside it.
// Returns the path written to.
func Save(tmpl map[string]any, path string) (string, error) {
	if path == "" {
		path = defaultTemplateFile
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, defaultTemplateFile)
	}
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// Load reads a template from a JSON file and strips any excluded fields.
// When path is empty, tries cwd, binary directory, platform config dir, then ~/.config/jira-thing.
func Load(path string) (map[string]any, error) {
	if path != "" {
		return loadAndStrip(path)
	}
	candidates := candidatePaths()
	for _, p := range candidates {
		tmpl, err := loadAndStrip(p)
		if err == nil {
			return tmpl, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("template not found; tried: %s", strings.Join(candidates, ", "))
}

// loadAndStrip reads a template file and strips excluded fields.
func loadAndStrip(path string) (map[string]any, error) {
	tmpl, err := loadFile(path)
	if err != nil {
		return nil, err
	}
	return StripExcludedFields(tmpl), nil
}

// CandidatePathsFunc allows tests to override fallback path resolution.
var CandidatePathsFunc = defaultCandidatePaths

// candidatePaths returns ordered fallback locations for the default template file.
func candidatePaths() []string {
	return CandidatePathsFunc()
}

func defaultCandidatePaths() []string {
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
