package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

// sprintAddJSON is the -json output of sprint add.
type sprintAddJSON struct {
	SprintID   int      `json:"sprintId"`
	SprintName string   `json:"sprintName,omitempty"`
	Keys       []string `json:"keys"`
	Status     string   `json:"status"`
}

// sprintAddFlags holds runSprintAdd's parsed flags and positional arguments.
type sprintAddFlags struct {
	boards boardFlags
	sprint string
	keys   []string
	dryRun bool
	json   bool
}

// parseSprintAddFlags parses runSprintAdd's flag set, exiting on a parse error.
func parseSprintAddFlags(args []string) sprintAddFlags {
	fs := flag.NewFlagSet("sprint add", flag.ExitOnError)
	f := sprintAddFlags{boards: addBoardFlags(fs)}
	fs.BoolVar(&f.dryRun, "dry-run", false, "Resolve the sprint and show the request without moving anything")
	fs.BoolVar(&f.json, "json", false, "Print {sprintId, sprintName, keys, status} as JSON")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 2 {
		fatal(sprintAddUsage)
	}
	f.sprint = fs.Arg(0)
	f.keys = fs.Args()[1:]
	return f
}

// runSprintAdd moves tickets into the current, next or a numbered sprint.
func runSprintAdd(args []string) {
	f := parseSprintAddFlags(args)
	conn := mustConnect()
	req := ops.AddToSprintRequest{Scope: f.boards.scope(), Sprint: f.sprint, Keys: f.keys}
	res, err := ops.AddToSprint(conn, req, f.dryRun)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	reportAddToSprint(conn, res, f.json)
}

// reportAddToSprint prints the move, or the request a dry run would send.
func reportAddToSprint(conn api.JiraConnection, res ops.AddToSprintResult, jsonOut bool) {
	if jsonOut {
		printJSONLine(sprintAddJSON{SprintID: res.Sprint.ID, SprintName: res.Sprint.Name, Keys: res.Keys, Status: res.Status})
		return
	}
	if res.Status == ops.StatusDryRun {
		printDryRun("POST", fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", conn.BaseURL, res.Sprint.ID), res.Payload)
		return
	}
	fmt.Printf("%s %d ticket(s) to sprint %s\n", tui.SuccessStyle.Render("Added"), len(res.Keys),
		termsafe.Line(sprintTitle(res.Sprint)))
}

// menuListSprints is the menu's "Sprints" action: list a board's sprints,
// then optionally the issues in one of them.
func menuListSprints(conn api.JiraConnection) {
	scope, ok := menuSprintScope()
	if !ok {
		return
	}
	board, sprints, err := ops.ListSprints(conn, scope, "")
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	printSprints(board, sprints)
	which := promptLine("View issues in sprint (current, prev, next or ID; blank to skip): ")
	if which == "" {
		return
	}
	scope.BoardID = board.ID
	got, err := ops.ListSprintIssues(conn, scope, which, "")
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	printTasks(got.Issues)
}

// menuAddToSprint is the menu's "Add to Sprint" action.
func menuAddToSprint(conn api.JiraConnection) {
	scope, ok := menuSprintScope()
	if !ok {
		return
	}
	which := promptLine("Sprint (current, next or ID): ")
	keys := template.SplitLabels(strings.ToUpper(promptLine("Ticket keys (comma-separated): ")))
	if which == "" || len(keys) == 0 {
		return
	}
	res, err := ops.AddToSprint(conn, ops.AddToSprintRequest{Scope: scope, Sprint: which, Keys: keys}, false)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	reportAddToSprint(conn, res, false)
}

// menuSprintScope asks for a board ID, blank meaning the configured board or
// the project's only sprint board.
func menuSprintScope() (ops.SprintScope, bool) {
	board := 0
	if answer := promptLine("Board ID (blank for the configured board or project): "); answer != "" {
		id, err := strconv.Atoi(answer)
		if err != nil {
			printMenuErr("invalid board ID: %v", err)
			return ops.SprintScope{}, false
		}
		board = id
	}
	scope, err := configScope(board, "")
	if err != nil {
		printMenuErr("%v", err)
		return ops.SprintScope{}, false
	}
	return scope, true
}
