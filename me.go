package main

import (
	"flag"
	"fmt"

	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
)

// runMe prints the signed-in user's email, or account ID when Jira hides the
// email; -json prints the whole record.
func runMe(args []string) {
	fs := flag.NewFlagSet("me", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Print {accountId, displayName, emailAddress} as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		fatal("usage: jira-thing me [-json]")
	}
	me, err := ops.WhoAmI(mustConnect())
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	switch {
	case *jsonOut:
		printJSONLine(me)
	case me.Email != "":
		fmt.Println(me.Email)
	default:
		fmt.Println(me.AccountID)
	}
}
