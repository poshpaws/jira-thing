package main

import (
	"errors"
	"fmt"
	"os"

	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/termsafe"
)

// createBase loads what a single create starts from: nothing with
// -no-template, the named template with -T, else the template file (-t, or
// the first on the search path, which is announced on stderr since the
// command line doesn't show it).
func createBase(flags createFlags, cfg config.Config) map[string]any {
	switch {
	case flags.noTemplate && (flags.templatePath != "" || flags.named != ""):
		fatal("-no-template can't be combined with -t or -T")
	case flags.noTemplate:
		return map[string]any{}
	case flags.named != "":
		return namedCreateBase(flags, cfg)
	}
	tmpl, from := loadCreateTemplate(flags.templatePath, cfg.TemplatesDir, flags.project != "")
	if flags.templatePath == "" && from != "" && !flags.json {
		fmt.Fprintf(os.Stderr, "Using template: %s (pass -no-template to start from flags alone)\n", termsafe.Line(from))
	}
	return tmpl
}

// namedCreateBase loads the -T template, refusing -t alongside it.
func namedCreateBase(flags createFlags, cfg config.Config) map[string]any {
	if flags.templatePath != "" {
		fatal("pass -t (a template file) or -T (a named template), not both")
	}
	tmpl, err := ops.LoadTemplate(cfg, flags.named)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
	}
	return tmpl
}

// loadCreateTemplate loads the template at path, or searches templatesDir
// then the usual search path, exiting with a pointer to the search path
// (and how to specify one directly) when none was found.
// With allowNone (a -project flag was given) and no path, a missing default
// template yields an empty one, so a ticket can be created from flags alone.
// It also returns the file loaded, empty when none was.
func loadCreateTemplate(path, templatesDir string, allowNone bool) (map[string]any, string) {
	tmpl, from, err := template.LoadFromDir(path, templatesDir)
	if path == "" && allowNone && errors.Is(err, template.ErrTemplateNotFound) {
		return map[string]any{}, ""
	}
	if err != nil {
		if path == "" {
			fatal("no template found in search path.\n\nCreate one first:\n  jira-thing template <TICKET-KEY> [-o ticket_template.json]\n\nThen place ticket_template.json in one of:\n  - \"templates_dir\" from config, if set\n  - current directory\n  - same directory as the jira-thing binary\n  - $XDG_CONFIG_HOME/jira-thing/\n  - ~/.config/jira-thing/\n\nOr specify a path directly:\n  jira-thing create -t /path/to/template.json")
		}
		fatal("loading template: %v", err)
	}
	return tmpl, from
}
