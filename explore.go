package main

import (
	"flag"
	"fmt"
	"strconv"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

// showExplorerFn launches the explorer TUI; replaced in tests.
var showExplorerFn = tui.ShowExplorer

// pauseFn waits for the user after a quick action, so its output isn't
// hidden when the explorer redraws; replaced in tests.
var pauseFn = func() { promptLine("Press enter to go back to the explorer… ") }

// explorer is what an explorer run needs: its title, sidebar groups, how to
// load a group's issues, and which group to open first.
type explorer struct {
	title  string
	groups []tui.ExplorerGroup
	load   tui.GroupLoader
	start  int
}

// runExplorer shows the explorer, carries out each quick action the user
// picks, and reopens it on the same group, until the user quits.
func runExplorer(conn api.JiraConnection, ex explorer) {
	start := ex.start
	for {
		res, err := showExplorerFn(ex.title, ex.groups, ex.load, start)
		if err != nil {
			fatal("%v", err)
			return
		}
		if res.Action == tui.ActionNone {
			return
		}
		performQuickAction(conn, res.Action, res.Key)
		pauseFn()
		start = res.Group
	}
}

// runEpicExplore opens the explorer on a project's epics.
func runEpicExplore(args []string) {
	fs := flag.NewFlagSet("epic explore", flag.ExitOnError)
	project := fs.String("p", "", `Project key (default: "project" in config)`)
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal("usage: jira-thing epic explore [-p PROJECT]")
	}
	cfg, err := config.Load()
	if err != nil {
		fatal("loading config: %v", err)
	}
	conn := mustConnect()
	ex, err := epicExplorer(conn, ops.ProjectOrConfig(cfg, *project))
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	runExplorer(conn, ex)
}

// epicExplorer lists a project's epics as explorer groups.
func epicExplorer(conn api.JiraConnection, project string) (explorer, error) {
	epics, err := ops.ListEpics(conn, project)
	if err != nil {
		return explorer{}, err
	}
	groups := make([]tui.ExplorerGroup, len(epics))
	for i, t := range issuesToTickets(epics) {
		groups[i] = tui.ExplorerGroup{ID: t.Key, Title: t.Key + " " + t.Summary, Detail: t.Status}
	}
	load := func(g tui.ExplorerGroup) ([]tui.Ticket, error) {
		issues, err := ops.ListEpicIssues(conn, g.ID)
		return issuesToTickets(issues), err
	}
	return explorer{title: "Epics in " + project, groups: groups, load: load}, nil
}

// runSprintExplore opens the explorer on a board's sprints.
func runSprintExplore(args []string) {
	fs := flag.NewFlagSet("sprint explore", flag.ExitOnError)
	boards := addBoardFlags(fs)
	all := fs.Bool("all", false, "Include closed sprints (default: active and future only)")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal("usage: jira-thing sprint explore [-b BOARD] [-p PROJECT] [-all]")
	}
	conn := mustConnect()
	ex, err := sprintExplorer(conn, boards.scope(), *all)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	runExplorer(conn, ex)
}

// sprintExplorer lists a board's sprints as explorer groups, opening on the
// active sprint.
func sprintExplorer(conn api.JiraConnection, scope ops.SprintScope, all bool) (explorer, error) {
	state := "active,future"
	if all {
		state = ""
	}
	board, sprints, err := ops.ListSprints(conn, scope, state)
	if err != nil {
		return explorer{}, err
	}
	ex := explorer{title: "Sprints on board " + strconv.Itoa(board.ID), groups: make([]tui.ExplorerGroup, len(sprints))}
	for i, sp := range sprints {
		ex.groups[i] = tui.ExplorerGroup{ID: strconv.Itoa(sp.ID), Title: sp.Name, Detail: sp.State}
		if sp.State == "active" {
			ex.start = i
		}
	}
	scope.BoardID = board.ID
	ex.load = func(g tui.ExplorerGroup) ([]tui.Ticket, error) {
		got, err := ops.ListSprintIssues(conn, scope, g.ID, "")
		return issuesToTickets(got.Issues), err
	}
	return ex, nil
}

// menuEpicExplorer is the menu's "Epic Explorer" action.
func menuEpicExplorer(conn api.JiraConnection) {
	cfg, err := config.Load()
	if err != nil {
		printMenuErr("loading config: %v", err)
		return
	}
	ex, err := epicExplorer(conn, ops.ProjectOrConfig(cfg, promptLine("Project key (blank for config default): ")))
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	runExplorer(conn, ex)
}

// menuSprintExplorer is the menu's "Sprint Explorer" action.
func menuSprintExplorer(conn api.JiraConnection) {
	scope, ok := menuSprintScope()
	if !ok {
		return
	}
	ex, err := sprintExplorer(conn, scope, false)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	fmt.Println()
	runExplorer(conn, ex)
}
