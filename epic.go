package main

import (
	"flag"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/tui"
)

// runEpic dispatches epic sub-commands: list, describe, create, add, remove.
func runEpic(args []string) {
	if len(args) < 1 {
		fatal("usage: jira-thing epic <list|describe|create|add|remove> [options]")
	}
	switch args[0] {
	case "list":
		runEpicList(args[1:])
	case "describe":
		runEpicDescribe(args[1:])
	case "create":
		runEpicCreate(args[1:])
	case "add":
		runEpicAdd(args[1:])
	case "remove":
		runEpicRemove(args[1:])
	default:
		fatal("unknown epic sub-command: %s\nusage: jira-thing epic <list|describe|create|add|remove> [options]", args[0])
	}
}

// runEpicList lists every epic in a project (default: the "project" set in
// config; override with -p).
func runEpicList(args []string) {
	fs := flag.NewFlagSet("epic list", flag.ExitOnError)
	project := fs.String("p", "", "Project key (defaults to \"project\" in config)")
	if err := fs.Parse(args); err != nil {
		fatal("usage: jira-thing epic list [-p PROJECT]")
	}
	projectKey := *project
	if projectKey == "" {
		cfg, err := config.Load()
		if err != nil || cfg.Project == "" {
			fatal("no project specified: pass -p PROJECT or set \"project\" in ~/.config/jira-thing/jira-thing.json")
		}
		projectKey = cfg.Project
	}
	conn := mustConnect()
	result, err := api.FetchEpics(conn, projectKey)
	if err != nil {
		fatal("listing epics: %v", err)
	}
	if len(result.Issues) == 0 {
		fmt.Printf("No epics found in %s.\n", projectKey)
		return
	}
	printTasks(result.Issues)
}

// runEpicDescribe lists the issues under a specific epic. Tries the "parent"
// field, then falls back to the classic "Epic Link" custom field.
func runEpicDescribe(args []string) {
	fs := flag.NewFlagSet("epic describe", flag.ExitOnError)
	if err := fs.Parse(args); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing epic describe <EPIC-KEY>")
	}
	conn := mustConnect()
	result, err := api.ListEpicIssues(conn, fs.Arg(0))
	if err != nil {
		fatal("listing epic issues: %v", err)
	}
	if len(result.Issues) == 0 {
		fmt.Println("No issues found under this epic.")
		return
	}
	printTasks(result.Issues)
}

// runEpicCreate creates an epic. On instances with a classic "Epic Name" custom
// field, it is populated; team-managed projects don't have this field.
func runEpicCreate(args []string) {
	fs := flag.NewFlagSet("epic create", flag.ExitOnError)
	project := fs.String("p", "", "Project key (required)")
	name := fs.String("n", "", "Epic name (required)")
	summary := fs.String("s", "", "Epic summary (required)")
	if err := fs.Parse(args); err != nil {
		fatal("usage: jira-thing epic create -p PROJECT -n NAME -s SUMMARY")
	}
	if *project == "" || *name == "" || *summary == "" {
		fatal("usage: jira-thing epic create -p PROJECT -n NAME -s SUMMARY")
	}
	conn := mustConnect()
	fields := api.BuildEpicFields(conn, *project, *name, *summary)
	result, err := api.CreateIssue(conn, fields)
	if err != nil {
		fatal("creating epic: %v", err)
	}
	key := getString(result, "key")
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Created epic:"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

// runEpicAdd adds issues to an epic.
func runEpicAdd(args []string) {
	fs := flag.NewFlagSet("epic add", flag.ExitOnError)
	if err := fs.Parse(args); err != nil || fs.NArg() < 2 {
		fatal("usage: jira-thing epic add <EPIC-KEY> <ISSUE-1> [ISSUE-2 ...]")
	}
	conn := mustConnect()
	epicKey := fs.Arg(0)
	issueKeys := fs.Args()[1:]
	if err := api.AddIssuesToEpic(conn, epicKey, issueKeys); err != nil {
		fatal("adding to epic: %v", err)
	}
	fmt.Printf("Added %s to %s\n", strings.Join(issueKeys, ", "), epicKey)
}

// runEpicRemove removes issues from their epic.
func runEpicRemove(args []string) {
	fs := flag.NewFlagSet("epic remove", flag.ExitOnError)
	if err := fs.Parse(args); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing epic remove <ISSUE-1> [ISSUE-2 ...]")
	}
	conn := mustConnect()
	issueKeys := fs.Args()
	if err := api.RemoveIssuesFromEpic(conn, issueKeys); err != nil {
		fatal("removing from epic: %v", err)
	}
	fmt.Printf("Removed %s from their epic\n", strings.Join(issueKeys, ", "))
}
