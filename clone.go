package main

import (
	"flag"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

const cloneUsage = "usage: jira-thing clone <TICKET-KEY> [-summary S] [-priority P] [-assignee WHO] [-labels a,b] [-replace find:replace]... [-dry-run] [-json]"

// cloneFlags holds runClone's parsed flags and positional argument.
type cloneFlags struct {
	req    ops.CloneRequest
	dryRun bool
	json   bool
}

// parseCloneFlags parses runClone's flag set, exiting on a parse error or a
// malformed -replace.
func parseCloneFlags(args []string) cloneFlags {
	fs := flag.NewFlagSet("clone", flag.ExitOnError)
	var f cloneFlags
	fs.StringVar(&f.req.Summary, "summary", "", `Summary for the copy (default: "CLONE - " + the source summary)`)
	fs.StringVar(&f.req.Priority, "priority", "", "Priority for the copy (default: the source's)")
	fs.StringVar(&f.req.Assignee, "assignee", "", "me, unassign, a name, an email or an account ID (default: unassigned)")
	labels := fs.String("labels", "", "Comma-separated labels replacing the source's")
	var replace stringList
	fs.Var(&replace, "replace", "find:replace applied to the summary and description (repeatable)")
	fs.BoolVar(&f.dryRun, "dry-run", false, "Show the create request without creating anything")
	fs.BoolVar(&f.json, "json", false, "Print {key, url, status} as JSON")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() != 1 {
		fatal(cloneUsage)
	}
	f.req.SourceKey = fs.Arg(0)
	if *labels != "" {
		f.req.Labels = template.SplitLabels(*labels)
	}
	for _, pair := range replace {
		r, err := ops.ParseReplacement(pair)
		if err != nil {
			fatal("%v", err)
		}
		f.req.Replace = append(f.req.Replace, r)
	}
	return f
}

// runClone copies a ticket, with optional overrides and text replacements.
func runClone(args []string) {
	f := parseCloneFlags(args)
	conn := mustConnect()
	res, err := ops.CloneTicket(conn, f.req, f.dryRun)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	reportClone(conn, res, f.json)
}

// reportClone prints the new ticket, or the request a dry run would send.
func reportClone(conn api.JiraConnection, res ops.CloneResult, jsonOut bool) {
	switch {
	case jsonOut:
		printJSONLine(jsonResult{Key: res.Key, URL: res.URL, Status: res.Status})
	case res.Status == ops.StatusDryRun:
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint, res.Payload)
	default:
		fmt.Printf("%s %s → %s\nURL: %s\n", tui.SuccessStyle.Render("Cloned"), res.SourceKey, tui.KeyStyle.Render(res.Key), res.URL)
	}
}

// menuCloneTicket is the menu's "Clone Ticket" action.
func menuCloneTicket(conn api.JiraConnection) {
	sourceKey := pickTicketKey(conn, "Source ticket key: ")
	if sourceKey == "" {
		fmt.Println("Cancelled.")
		return
	}
	req := ops.CloneRequest{SourceKey: sourceKey, Summary: promptLine("Summary (blank for default): ")}
	res, err := ops.CloneTicket(conn, req, false)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	reportClone(conn, res, false)
}
