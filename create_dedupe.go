package main

import (
	"fmt"

	"jira-thing/internal/ops"
	"jira-thing/internal/tui"
)

// reportDuplicate prints the open ticket that made a create unnecessary, as
// JSON when jsonOut is set, otherwise as styled text.
func reportDuplicate(res ops.CreateResult, jsonOut bool) {
	if jsonOut {
		printJSONResult(jsonResult{Key: res.Key, URL: res.URL, Status: res.Status})
		return
	}
	fmt.Printf("%s %s already has this summary and is still open; nothing created\n",
		tui.SuccessStyle.Render("Skipped:"), tui.KeyStyle.Render(res.Key))
	fmt.Printf("URL: %s\n", res.URL)
}
