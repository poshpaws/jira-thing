package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const templateExt = ".json"

// ErrBadTemplateName means a template name could reach outside the templates
// directory or isn't a plain file name.
var ErrBadTemplateName = errors.New(`template names use only letters, digits, "-" and "_"`)

// templateName is what a named template may be called; it keeps names to a
// single path element, since agents pass them too.
var templateName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// NamedDir is where named templates live: templatesDir (the templates_dir
// setting) when set, else ~/.config/jira-thing/templates.
func NamedDir(templatesDir string) string {
	if templatesDir != "" {
		return ExpandHome(templatesDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "jira-thing", "templates")
}

// NamedPath returns the file for template name in dir, rejecting unsafe names.
func NamedPath(dir, name string) (string, error) {
	if !templateName.MatchString(name) {
		return "", fmt.Errorf("%w: %q", ErrBadTemplateName, name)
	}
	return filepath.Join(dir, name+templateExt), nil
}

// LoadNamed loads template name from dir, stripping excluded fields. A
// missing file returns an error satisfying os.IsNotExist.
func LoadNamed(dir, name string) (map[string]any, error) {
	path, err := NamedPath(dir, name)
	if err != nil {
		return nil, err
	}
	return loadAndStrip(path)
}

// ListNamed returns the names of the templates in dir, sorted. A missing
// directory has none; files whose names aren't valid template names are skipped.
func ListNamed(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading templates directory %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), templateExt)
		if ok && !e.IsDir() && templateName.MatchString(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}
