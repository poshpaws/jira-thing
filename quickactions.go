package main

import (
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

// quickActions carry out the actions chosen in the ticket table or the
// explorer, by kind.
var quickActions = map[tui.TableActionKind]func(api.JiraConnection, string){
	tui.ActionView:       viewTicket,
	tui.ActionOpen:       openTicket,
	tui.ActionTransition: menuTransitionTicket,
	tui.ActionCopyKey:    func(_ api.JiraConnection, key string) { copyText("key", key) },
	tui.ActionCopyURL:    func(conn api.JiraConnection, key string) { copyText("url", browseURL(conn, key)) },
}

// performQuickAction carries out action on key; ActionNone does nothing.
func performQuickAction(conn api.JiraConnection, action tui.TableActionKind, key string) {
	if run, ok := quickActions[action]; ok {
		run(conn, key)
	}
}

// browseURL is a ticket's page in the browser.
func browseURL(conn api.JiraConnection, key string) string {
	return conn.BaseURL + "/browse/" + key
}

// viewTicket prints a ticket and its latest comment as rendered markdown,
// the same view as describe.
func viewTicket(conn api.JiraConnection, key string) {
	res, err := ops.DescribeTicket(conn, key, ops.DefaultDescribeComments)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	renderDescribeResult(res, false, false)
}

// openTicket opens a ticket in the browser.
func openTicket(conn api.JiraConnection, key string) {
	target := browseURL(conn, key)
	if err := openBrowser(target); err != nil {
		printMenuErr("opening browser: %v", err)
		return
	}
	fmt.Printf("Opened %s\n", target)
}

// copyText copies text to the clipboard; what names it in messages.
func copyText(what, text string) {
	if err := copyToClipboard(text); err != nil {
		printMenuErr("copying %s: %v", what, err)
		return
	}
	fmt.Printf("Copied %s to clipboard\n", text)
}
