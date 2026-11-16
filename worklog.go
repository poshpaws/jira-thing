package main

import (
	"flag"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

const worklogUsage = `usage: jira-thing worklog add <TICKET-KEY> <DURATION, e.g. "2h 30m"> [-comment TEXT] [-dry-run] [-json]`

// worklogJSON is the -json output of worklog add.
type worklogJSON struct {
	jsonResult
	TimeSpent string `json:"timeSpent"`
}

// runWorklog dispatches the worklog sub-commands.
func runWorklog(args []string) {
	if len(args) < 1 {
		fatal(worklogUsage)
	}
	switch args[0] {
	case "add":
		runWorklogAdd(args[1:])
	default:
		fatal("unknown worklog sub-command: %s\n%s", args[0], worklogUsage)
	}
}

// runWorklogAdd logs time against a ticket. The duration may be quoted or
// given as separate words: `worklog add PROJ-1 2h 30m`.
func runWorklogAdd(args []string) {
	fs := flag.NewFlagSet("worklog add", flag.ExitOnError)
	comment := fs.String("comment", "", "Markdown comment describing the work")
	dryRun := fs.Bool("dry-run", false, "Check the duration and show the request without logging anything")
	jsonOut := fs.Bool("json", false, "Print {key, url, status, timeSpent} as JSON")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 2 {
		fatal(worklogUsage)
	}
	req := ops.WorklogRequest{Key: fs.Arg(0), TimeSpent: strings.Join(fs.Args()[1:], " "), Comment: *comment}
	conn := mustConnect()
	res, err := ops.LogWork(conn, req, *dryRun)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	reportWorklog(conn, res, *jsonOut)
}

// reportWorklog prints the time logged, or the request a dry run would send.
func reportWorklog(conn api.JiraConnection, res ops.WorklogResult, jsonOut bool) {
	switch {
	case jsonOut:
		printJSONLine(worklogJSON{jsonResult: jsonResult{Key: res.Key, URL: res.URL, Status: res.Status}, TimeSpent: res.TimeSpent})
	case res.Status == ops.StatusDryRun:
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint+"/"+res.Key+"/worklog", res.Payload)
	default:
		fmt.Printf("%s %s on %s\n", tui.SuccessStyle.Render("Logged"), res.TimeSpent, tui.KeyStyle.Render(res.Key))
	}
}

// menuAddWorklog is the menu's "Log Work" action.
func menuAddWorklog(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	req := ops.WorklogRequest{Key: key, TimeSpent: promptLine("Time spent (e.g. 2h, 1d 3h): ")}
	req.Comment = promptLine("Comment (blank to skip): ")
	res, err := ops.LogWork(conn, req, false)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	reportWorklog(conn, res, false)
}
