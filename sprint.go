package main

import (
	"errors"
	"flag"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

const (
	sprintUsage     = "usage: jira-thing sprint <list|add|explore> [options]"
	sprintListUsage = "usage: jira-thing sprint list [-b BOARD] [-p PROJECT] [-state STATE] [-current|-prev|-next|-sprint ID] [-jql JQL] [-json]"
	sprintAddUsage  = "usage: jira-thing sprint add <current|next|SPRINT_ID> <TICKET-KEY...> [-b BOARD] [-p PROJECT] [-dry-run] [-json]"
)

// errOneSprint means more than one sprint selector flag was given.
var errOneSprint = errors.New("pick one of -current, -prev, -next or -sprint")

// runSprint dispatches the sprint sub-commands.
func runSprint(args []string) {
	if len(args) < 1 {
		fatal(sprintUsage)
	}
	switch args[0] {
	case "list":
		runSprintList(args[1:])
	case "add":
		runSprintAdd(args[1:])
	case "explore":
		runSprintExplore(args[1:])
	default:
		fatal("unknown sprint sub-command: %s\n%s", args[0], sprintUsage)
	}
}

// boardFlags are the -b and -p flags every sprint sub-command takes.
type boardFlags struct {
	board   *int
	project *string
}

// addBoardFlags defines -b and -p on fs.
func addBoardFlags(fs *flag.FlagSet) boardFlags {
	return boardFlags{
		board:   fs.Int("b", 0, "Board ID (default: \"board\" in config, else the project's only sprint board)"),
		project: fs.String("p", "", "Project whose sprint board to use (default: \"project\" in config)"),
	}
}

// scope turns the flags into a sprint scope, exiting if the config is unreadable.
func (b boardFlags) scope() ops.SprintScope {
	scope, err := configScope(*b.board, *b.project)
	if err != nil {
		fatal("%v", err)
	}
	return scope
}

// configScope builds a sprint scope, falling back to the config file.
func configScope(board int, project string) (ops.SprintScope, error) {
	cfg, err := config.Load()
	if err != nil {
		return ops.SprintScope{}, fmt.Errorf("loading config: %w", err)
	}
	return ops.ScopeFromConfig(cfg, board, project), nil
}

// sprintListFlags holds runSprintList's parsed flags.
type sprintListFlags struct {
	boards boardFlags
	state  string
	which  string
	jql    string
	json   bool
}

// parseSprintListFlags parses runSprintList's flag set, exiting on a parse error.
func parseSprintListFlags(args []string) sprintListFlags {
	fs := flag.NewFlagSet("sprint list", flag.ExitOnError)
	f := sprintListFlags{boards: addBoardFlags(fs)}
	fs.StringVar(&f.state, "state", "", "Only list sprints in these states: active, future, closed (comma-separated)")
	current := fs.Bool("current", false, "List the issues in the active sprint")
	prev := fs.Bool("prev", false, "List the issues in the last closed sprint")
	next := fs.Bool("next", false, "List the issues in the next future sprint")
	id := fs.String("sprint", "", "List the issues in this sprint ID")
	fs.StringVar(&f.jql, "jql", "", "Narrow a sprint's issues with a JQL filter")
	fs.BoolVar(&f.json, "json", false, "Print the result as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal(sprintListUsage)
	}
	which, err := sprintSelectorFlag(*current, *prev, *next, *id)
	if err != nil {
		fatal("%v\n%s", err, sprintListUsage)
	}
	f.which = which
	return f
}

// sprintSelectorFlag turns the selector flags into one selector, "" for none.
func sprintSelectorFlag(current, prev, next bool, id string) (string, error) {
	chosen := []string{}
	for sel, on := range map[string]bool{ops.SprintCurrent: current, ops.SprintPrev: prev, ops.SprintNext: next, id: id != ""} {
		if on {
			chosen = append(chosen, sel)
		}
	}
	switch len(chosen) {
	case 0:
		return "", nil
	case 1:
		return chosen[0], nil
	}
	return "", errOneSprint
}

// runSprintList lists a board's sprints or, with a sprint selector, the
// issues in that sprint.
func runSprintList(args []string) {
	f := parseSprintListFlags(args)
	conn := mustConnect()
	if f.which != "" {
		listSprintIssues(conn, f)
		return
	}
	board, sprints, err := ops.ListSprints(conn, f.boards.scope(), f.state)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	if f.json {
		printJSONLine(sprints)
		return
	}
	printSprints(board, sprints)
}

// listSprintIssues prints the issues in the selected sprint.
func listSprintIssues(conn api.JiraConnection, f sprintListFlags) {
	got, err := ops.ListSprintIssues(conn, f.boards.scope(), f.which, f.jql)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	if f.json {
		printTicketsJSON(conn, got.Issues)
		return
	}
	fmt.Println(tui.HeadingStyle.Render("Sprint " + termsafe.Line(sprintTitle(got.Sprint))))
	if len(got.Issues) == 0 {
		fmt.Println("No issues found.")
		return
	}
	printTasks(got.Issues)
}

// printSprints prints one line per sprint: ID, name, state and dates.
func printSprints(board api.Board, sprints []api.Sprint) {
	if len(sprints) == 0 {
		fmt.Printf("No sprints found on board %d.\n", board.ID)
		return
	}
	if board.Name != "" {
		fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Board %d: %s", board.ID, termsafe.Line(board.Name))))
	}
	for _, sp := range sprints {
		fmt.Printf("  %s %s %s %s\n", tui.KeyStyle.Render(fmt.Sprintf("[%d]", sp.ID)), termsafe.Line(sp.Name),
			tui.StatusStyle.Render(termsafe.Line(sp.State)), tui.DateStyle.Render(sprintDates(sp)))
	}
}

// sprintTitle is "name (id)", or just the ID when the sprint was given by ID.
func sprintTitle(sp api.Sprint) string {
	if sp.Name == "" {
		return fmt.Sprint(sp.ID)
	}
	return fmt.Sprintf("%s (%d)", sp.Name, sp.ID)
}

// sprintDates is "start → end" for a started sprint, empty otherwise.
func sprintDates(sp api.Sprint) string {
	const dateLen = len("2006-01-02")
	if len(sp.StartDate) < dateLen || len(sp.EndDate) < dateLen {
		return ""
	}
	return termsafe.Line(sp.StartDate[:dateLen] + " → " + sp.EndDate[:dateLen])
}
