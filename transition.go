package main

import (
	"flag"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/tui"
)

const transitionUsage = "usage: jira-thing transition <TICKET-KEY> [STATE] [-comment C] [-resolution R] [-assignee me|unassign|NAME|EMAIL|ACCOUNT_ID] [-dry-run]\n" +
	"  omit STATE to list the states available from the ticket's current status"

// transitionFlags holds runTransition's parsed flags and positional arguments.
type transitionFlags struct {
	key        string
	state      string
	comment    string
	resolution string
	assignee   string
	dryRun     bool
}

// parseTransitionFlags parses runTransition's flag set, exiting on a parse error.
func parseTransitionFlags(args []string) transitionFlags {
	fs := flag.NewFlagSet("transition", flag.ExitOnError)
	comment := fs.String("comment", "", "Optional comment (markdown) to add as part of the transition")
	resolution := fs.String("resolution", "", `Optional resolution name (e.g. "Fixed", "Won't Do")`)
	assignee := fs.String("assignee", "", `Optional assignee to set: "me", "unassign", a name, an email, or a Jira account ID`)
	dryRun := fs.Bool("dry-run", false, "Show the resolved transition without applying it")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 1 {
		fatal(transitionUsage)
	}
	var state string
	if fs.NArg() > 1 {
		state = fs.Arg(1)
	}
	return transitionFlags{fs.Arg(0), state, *comment, *resolution, *assignee, *dryRun}
}

// runTransition moves a ticket to a new workflow state non-interactively — the
// scriptable complement to "state", which requires the TUI picker. Omitting
// STATE lists the states available from the ticket's current status, the
// same list the TUI would offer.
func runTransition(args []string) {
	flags := parseTransitionFlags(args)

	conn := mustConnect()
	if flags.state == "" {
		transitions, err := ops.ListTransitions(conn, flags.key)
		if err != nil {
			fatal("%v", err)
		}
		listTransitions(flags.key, transitions)
		return
	}

	res, err := ops.TransitionTicket(conn, ops.TransitionRequest{
		Key:        flags.key,
		State:      flags.state,
		Comment:    flags.comment,
		Resolution: flags.resolution,
		Assignee:   flags.assignee,
	}, flags.dryRun)
	if err != nil {
		fatal("%v", err)
	}
	if res.Status == ops.StatusDryRun {
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint+"/"+res.Key+"/transitions", res.Payload)
		return
	}
	fmt.Printf("%s %s %s %s\n",
		tui.SuccessStyle.Render("Moved"), tui.KeyStyle.Render(res.Key), tui.SuccessStyle.Render("to"), res.Transition.Name)
}

// listTransitions prints the states available from the ticket's current status.
func listTransitions(key string, transitions []api.Transition) {
	if len(transitions) == 0 {
		fmt.Printf("No transitions available for %s.\n", key)
		return
	}
	fmt.Print(ops.FormatTransitions(key, transitions))
}
