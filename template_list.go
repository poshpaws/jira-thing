package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

// runTemplateList prints the named templates "create -T" accepts, with where
// each comes from, as JSON with -json.
func runTemplateList(args []string) {
	fs := flag.NewFlagSet("template list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Print [{name, source}] as JSON")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		fatal("loading config: %v", err)
	}
	infos, err := ops.ListTemplates(cfg)
	if err != nil {
		fatal("%v", err)
	}
	if *jsonOut {
		printTemplateListJSON(infos)
		return
	}
	fmt.Printf("Templates in %s (use with: create -T NAME)\n", template.NamedDir(cfg.TemplatesDir))
	for _, info := range infos {
		fmt.Printf("  %-20s %s\n", tui.KeyStyle.Render(info.Name), info.Source)
	}
}

// printTemplateListJSON prints infos as an indented JSON array.
func printTemplateListJSON(infos []ops.TemplateInfo) {
	body, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		fatal("marshalling templates: %v", err)
	}
	fmt.Println(string(body))
}
