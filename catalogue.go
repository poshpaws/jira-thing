package main

import (
	"flag"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

const (
	boardListUsage   = "usage: jira-thing board list [-p PROJECT] [-json]"
	projectListUsage = "usage: jira-thing project list [-json]"
	releaseListUsage = "usage: jira-thing release list [-p PROJECT] [-all] [-unreleased] [-json]"
)

// runBoard dispatches the board sub-commands.
func runBoard(args []string) {
	if len(args) < 1 {
		fatal(boardListUsage)
	}
	switch args[0] {
	case "list":
		runBoardList(args[1:])
	default:
		fatal("unknown board sub-command: %s\n%s", args[0], boardListUsage)
	}
}

// runProject dispatches the project sub-commands.
func runProject(args []string) {
	if len(args) < 1 {
		fatal(projectListUsage)
	}
	switch args[0] {
	case "list":
		runProjectList(args[1:])
	default:
		fatal("unknown project sub-command: %s\n%s", args[0], projectListUsage)
	}
}

// runRelease dispatches the release sub-commands.
func runRelease(args []string) {
	if len(args) < 1 {
		fatal(releaseListUsage)
	}
	switch args[0] {
	case "list":
		runReleaseList(args[1:])
	default:
		fatal("unknown release sub-command: %s\n%s", args[0], releaseListUsage)
	}
}

// runBoardList lists Agile boards, all of them or one project's.
func runBoardList(args []string) {
	fs := flag.NewFlagSet("board list", flag.ExitOnError)
	project := fs.String("p", "", "Only list this project's boards")
	jsonOut := fs.Bool("json", false, "Print the boards as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal(boardListUsage)
	}
	boards, err := ops.ListBoards(mustConnect(), *project)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	if *jsonOut {
		printJSONLine(boards)
		return
	}
	printBoards(boards)
}

// runProjectList lists every project the user can browse.
func runProjectList(args []string) {
	fs := flag.NewFlagSet("project list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Print the projects as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal(projectListUsage)
	}
	projects, err := ops.ListProjects(mustConnect())
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	if *jsonOut {
		printJSONLine(projects)
		return
	}
	printProjects(projects)
}

// runReleaseList lists a project's releases, newest release date first.
func runReleaseList(args []string) {
	fs := flag.NewFlagSet("release list", flag.ExitOnError)
	project := fs.String("p", "", "Project key (default: \"project\" in config)")
	all := fs.Bool("all", false, "Include archived releases")
	unreleased := fs.Bool("unreleased", false, "Only list releases not yet released")
	jsonOut := fs.Bool("json", false, "Print the releases as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal(releaseListUsage)
	}
	cfg, err := config.Load()
	if err != nil {
		fatal("loading config: %v", err)
	}
	filter := ops.ReleaseFilter{Project: ops.ProjectOrConfig(cfg, *project), IncludeArchived: *all, UnreleasedOnly: *unreleased}
	versions, err := ops.ListReleases(mustConnect(), filter)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	if *jsonOut {
		printJSONLine(versions)
		return
	}
	printReleases(versions)
}

// printBoards prints one line per board: ID, name and type.
func printBoards(boards []api.Board) {
	if len(boards) == 0 {
		fmt.Println("No boards found.")
		return
	}
	for _, b := range boards {
		fmt.Printf("  %s %s %s\n", tui.KeyStyle.Render(fmt.Sprintf("[%d]", b.ID)), termsafe.Line(b.Name),
			tui.DateStyle.Render(termsafe.Line(b.Type)))
	}
}

// printProjects prints one line per project: key and name.
func printProjects(projects []api.Project) {
	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return
	}
	for _, p := range projects {
		fmt.Printf("  %-12s %s\n", tui.KeyStyle.Render(termsafe.Line(p.Key)), termsafe.Line(p.Name))
	}
}

// printReleases prints one line per release: name, status and release date.
func printReleases(versions []api.ProjectVersion) {
	if len(versions) == 0 {
		fmt.Println("No releases found.")
		return
	}
	for _, v := range versions {
		fmt.Printf("  %s %s %s\n", tui.KeyStyle.Render(termsafe.Line(v.Name)),
			tui.StatusStyle.Render(ops.ReleaseStatus(v)), tui.DateStyle.Render(termsafe.Line(v.ReleaseDate)))
	}
}
