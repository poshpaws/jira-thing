package main

import (
	"flag"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

const (
	watchUsage    = "usage: jira-thing watch|unwatch <TICKET-KEY> [me|NAME|EMAIL|ACCOUNT_ID] [-dry-run] [-json]"
	watchersUsage = "usage: jira-thing watchers <TICKET-KEY> [-json]"
)

// watchJSON is the -json output of watch and unwatch.
type watchJSON struct {
	jsonResult
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName,omitempty"`
}

// runWatch makes someone (default: you) watch a ticket.
func runWatch(args []string) { runWatchChange(args, true) }

// runUnwatch stops someone (default: you) watching a ticket.
func runUnwatch(args []string) { runWatchChange(args, false) }

// runWatchChange parses watch/unwatch flags and makes the change. An
// ambiguous name opens a picker in a terminal, as with assign.
func runWatchChange(args []string, watch bool) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Resolve the user and show the change without making it")
	jsonOut := fs.Bool("json", false, "Print {key, url, status, accountId, displayName} as JSON")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 1 || fs.NArg() > 2 {
		fatal(watchUsage)
	}
	req := ops.WatchRequest{Key: fs.Arg(0), Who: fs.Arg(1), Watch: watch}
	conn := mustConnect()
	res, err := retryWithPick(func(picked *ops.User) (ops.WatchResult, error) {
		req.Picked = picked
		return ops.WatchTicket(conn, req, *dryRun)
	})
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	reportWatch(res, watch, *jsonOut)
}

// reportWatch prints who is (or would be) watching or not.
func reportWatch(res ops.WatchResult, watch, jsonOut bool) {
	if jsonOut {
		printJSONLine(watchJSON{jsonResult: jsonResult{Key: res.Key, URL: res.URL, Status: res.Status},
			AccountID: res.User.AccountID, DisplayName: res.User.DisplayName})
		return
	}
	prefix := ""
	if res.Status == ops.StatusDryRun {
		prefix = tui.HeadingStyle.Render("Dry run:") + " "
	}
	fmt.Printf("%s%s %s %s\n", prefix, termsafe.Line(ops.UserLabel(res.User)),
		tui.SuccessStyle.Render(ops.WatchVerb(res, watch)), tui.KeyStyle.Render(res.Key))
}

// runWatchers lists who is watching a ticket.
func runWatchers(args []string) {
	fs := flag.NewFlagSet("watchers", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Print the watchers as JSON")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() != 1 {
		fatal(watchersUsage)
	}
	users, err := ops.ListWatchers(mustConnect(), fs.Arg(0))
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	switch {
	case *jsonOut:
		printJSONLine(users)
	case len(users) == 0:
		fmt.Printf("Nobody is watching %s.\n", fs.Arg(0))
	default:
		for _, u := range users {
			fmt.Printf("  %s\n", termsafe.Line(ops.UserLabel(u)))
		}
	}
}

// menuWatch is the menu's "Watch / Unwatch" action: it flips whether you
// are watching the picked ticket.
func menuWatch(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		return
	}
	watching, err := ops.IsWatching(conn, key)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	res, err := ops.WatchTicket(conn, ops.WatchRequest{Key: key, Watch: !watching}, false)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	reportWatch(res, !watching, false)
}
