package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"

	"github.com/mattn/go-isatty"
)

const assignUsage = "usage: jira-thing assign <TICKET-KEY> <me|unassign|default|NAME|EMAIL|ACCOUNT_ID> [-dry-run] [-json]"

// isInteractiveFn reports whether a picker can be shown; replaced in tests.
var isInteractiveFn = func() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

// selectUserFn launches the user-picker TUI; replaced in tests.
var selectUserFn = pickUser

// assignFlags holds runAssign's parsed flags and positional arguments.
type assignFlags struct {
	req    ops.AssignRequest
	dryRun bool
	json   bool
}

// assignJSON is the -json output: the usual result plus who was assigned.
type assignJSON struct {
	jsonResult
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName,omitempty"`
}

// parseAssignFlags parses runAssign's flag set, exiting on a parse error.
func parseAssignFlags(args []string) assignFlags {
	fs := flag.NewFlagSet("assign", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Resolve the assignee and show it without changing the ticket")
	jsonOut := fs.Bool("json", false, "Print {key, url, status, accountId, displayName} as JSON")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 2 {
		fatal(assignUsage)
	}
	return assignFlags{req: ops.AssignRequest{Key: fs.Arg(0), Assignee: fs.Arg(1)}, dryRun: *dryRun, json: *jsonOut}
}

// runAssign sets a ticket's assignee by name, email or account ID. A name
// matching several users opens a picker in a terminal, and fails listing the
// candidates otherwise.
func runAssign(args []string) {
	flags := parseAssignFlags(args)
	conn := mustConnect()
	res, err := assignWithPicker(conn, flags)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	reportAssignResult(res, flags.json)
}

// assignWithPicker assigns, retrying with the user's pick when the name was
// ambiguous and a picker can be shown.
func assignWithPicker(conn api.JiraConnection, flags assignFlags) (ops.AssignResult, error) {
	return retryWithPick(func(picked *ops.User) (ops.AssignResult, error) {
		flags.req.Picked = picked
		return ops.AssignTicket(conn, flags.req, flags.dryRun)
	})
}

// retryWithPick runs do; when it fails on an ambiguous user name in a
// terminal, it shows the candidates and runs do again with the one picked.
func retryWithPick[T any](do func(picked *ops.User) (T, error)) (T, error) {
	res, err := do(nil)
	var amb *ops.AmbiguousUserError
	if !errors.As(err, &amb) || !isInteractiveFn() {
		return res, err
	}
	var zero T
	picked, ok, err := selectUserFn(amb.Candidates)
	if err != nil {
		return zero, err
	}
	if !ok {
		return zero, errors.New("cancelled")
	}
	return do(&picked)
}

// pickUser shows candidates in the menu picker and returns the chosen one.
func pickUser(candidates []ops.User) (ops.User, bool, error) {
	options := make([]tui.MenuOption, len(candidates))
	for i, u := range candidates {
		options[i] = tui.MenuOption{Label: termsafe.Line(u.DisplayName), Desc: termsafe.Line(u.Email + " " + u.AccountID)}
	}
	idx, cancelled, err := selectMenuOptionFn("Pick an assignee", options)
	if err != nil || cancelled {
		return ops.User{}, false, err
	}
	return candidates[idx], true, nil
}

// reportAssignResult prints who the ticket was (or would be) assigned to.
func reportAssignResult(res ops.AssignResult, jsonOut bool) {
	if jsonOut {
		printJSONLine(assignJSON{
			jsonResult:  jsonResult{Key: res.Key, URL: res.URL, Status: res.Status},
			AccountID:   res.Assignee.AccountID,
			DisplayName: res.Assignee.DisplayName,
		})
		return
	}
	verb := "Assigned"
	if res.Status == ops.StatusDryRun {
		verb = "Dry run: would assign"
	}
	fmt.Printf("%s %s to %s\n", tui.SuccessStyle.Render(verb), tui.KeyStyle.Render(res.Key),
		termsafe.Line(ops.AssigneeLabel(res.Assignee)))
	fmt.Printf("URL: %s\n", res.URL)
}

// menuAssign is the menu's "Assign Ticket" action: pick a ticket, name an
// assignee, and pick from the matches when the name is ambiguous.
func menuAssign(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		return
	}
	who := promptLine("Assign to (me, unassign, default, a name, email or account ID): ")
	if who == "" {
		return
	}
	res, err := assignWithPicker(conn, assignFlags{req: ops.AssignRequest{Key: key, Assignee: who}})
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	reportAssignResult(res, false)
}
