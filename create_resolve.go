package main

import (
	"fmt"
	"os"
	"strings"
)

// resolveCreateFields fills in summary and description on tmpl: an explicit
// -description-file always wins for the description; otherwise an existing
// template value is used; anything still missing is prompted for unless
// nonInteractive is set.
func resolveCreateFields(tmpl map[string]any, descriptionFile string, nonInteractive bool) error {
	if descriptionFile != "" {
		data, err := os.ReadFile(descriptionFile) // #nosec G304 -- user-supplied CLI input
		if err != nil {
			return fmt.Errorf("reading description file: %w", err)
		}
		tmpl["description"] = string(data)
	}

	summary, err := resolveSummary(tmpl, nonInteractive)
	if err != nil {
		return err
	}
	description := resolveDescription(tmpl, nonInteractive)

	tmpl["summary"] = summary
	if description != "" {
		tmpl["description"] = buildDescription(description)
	} else {
		delete(tmpl, "description")
	}
	return nil
}

// resolveSummary returns the template's summary if already set, otherwise
// prompts for one; in non-interactive mode a missing summary is a fatal error.
func resolveSummary(tmpl map[string]any, nonInteractive bool) (string, error) {
	if s, ok := tmpl["summary"].(string); ok && strings.TrimSpace(s) != "" {
		return s, nil
	}
	if nonInteractive {
		return "", fmt.Errorf(`summary is required: set "summary" in the template, or omit -non-interactive to be prompted`)
	}
	summary := promptLine("Enter ticket summary: ")
	if summary == "" {
		return "", fmt.Errorf("summary is required")
	}
	return summary, nil
}

// resolveDescription returns the template's description if already set,
// otherwise prompts for one. Description is optional, so a non-interactive
// run with none given simply creates the ticket without one.
func resolveDescription(tmpl map[string]any, nonInteractive bool) string {
	if d, ok := tmpl["description"].(string); ok && d != "" {
		return d
	}
	if nonInteractive {
		return ""
	}
	return promptLine("Enter ticket description: ")
}
