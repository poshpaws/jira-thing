package main

import (
	"flag"
	"fmt"

	"jira-thing/internal/api"
	mcpsrv "jira-thing/internal/mcpserver"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

const transitionUsage = "usage: jira-thing transition <TICKET-KEY> [STATE] [-comment C] [-resolution R] [-assignee me|unassign|ACCOUNT_ID] [-dry-run]\n" +
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
	assignee := fs.String("assignee", "", `Optional assignee to set: "me", "unassign", or a Jira account ID`)
	dryRun := fs.Bool("dry-run", false, "Show the resolved transition without applying it")
	if err := fs.Parse(reorderFlagsBeforeKey(args)); err != nil || fs.NArg() < 1 {
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
	transitions, err := api.FetchTransitions(conn, flags.key)
	if err != nil {
		fatal("fetching transitions for %s: %v", flags.key, err)
	}
	if flags.state == "" {
		listTransitions(flags.key, transitions)
		return
	}

	transition, err := mcpsrv.FindTransition(transitions, flags.state)
	if err != nil {
		fatal("%v\n\n%s", err, mcpsrv.FormatTransitions(flags.key, transitions))
	}
	opts, err := buildTransitionOptions(conn, flags)
	if err != nil {
		fatal("%v", err)
	}

	if flags.dryRun {
		printTransitionDryRun(conn, flags.key, transition, opts)
		return
	}
	applyTransition(conn, flags.key, transition, opts)
}

// applyTransition executes the transition and reports success.
func applyTransition(conn api.JiraConnection, key string, transition api.Transition, opts api.TransitionOptions) {
	if err := api.TransitionIssueWithOptions(conn, key, transition.ID, opts); err != nil {
		fatal("transitioning %s: %v", key, err)
	}
	fmt.Printf("%s %s %s %s\n",
		tui.SuccessStyle.Render("Moved"), tui.KeyStyle.Render(key), tui.SuccessStyle.Render("to"), transition.Name)
}

// listTransitions prints the states available from the ticket's current status.
func listTransitions(key string, transitions []api.Transition) {
	if len(transitions) == 0 {
		fmt.Printf("No transitions available for %s.\n", key)
		return
	}
	fmt.Print(mcpsrv.FormatTransitions(key, transitions))
}

// buildTransitionOptions translates transition flags into a TransitionOptions value.
func buildTransitionOptions(conn api.JiraConnection, flags transitionFlags) (api.TransitionOptions, error) {
	fields := map[string]any{}
	if flags.resolution != "" {
		fields["resolution"] = map[string]any{"name": flags.resolution}
	}
	if flags.assignee != "" {
		resolved, err := template.ResolveAssigneeArg(conn, flags.assignee)
		if err != nil {
			return api.TransitionOptions{}, err
		}
		fields["assignee"] = resolved
	}
	opts := api.TransitionOptions{Fields: fields}
	if flags.comment != "" {
		opts.Comment = markdownToADF(flags.comment)
	}
	return opts, nil
}

// printTransitionDryRun shows the transition request payload without applying it.
func printTransitionDryRun(conn api.JiraConnection, key string, transition api.Transition, opts api.TransitionOptions) {
	payload := map[string]any{"transition": map[string]any{"id": transition.ID, "name": transition.Name}}
	if len(opts.Fields) > 0 {
		payload["fields"] = opts.Fields
	}
	if opts.Comment != nil {
		payload["update"] = map[string]any{"comment": []map[string]any{{"add": map[string]any{"body": opts.Comment}}}}
	}
	printDryRun("POST", conn.BaseURL+api.IssueEndpoint+"/"+key+"/transitions", payload)
}
