package main

import (
	"encoding/json"
	"fmt"

	"jira-thing/internal/tui"
)

// printDryRun shows the HTTP method, target URL, and JSON body that would be sent
// for a mutating command, without making the call.
func printDryRun(method, url string, payload map[string]any) {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		body = []byte(fmt.Sprintf("<failed to render payload: %v>", err))
	}
	fmt.Println(tui.HeadingStyle.Render("Dry run — no API call made."))
	fmt.Printf("%s %s\n\n%s\n", method, url, body)
}

// jsonResult is the machine-readable shape -json prints for a single-ticket
// mutating command: the ticket key, its browse URL, and a short status word.
type jsonResult struct {
	Key    string `json:"key"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

// printJSONResult prints result as a single line of JSON, for scripts to parse.
func printJSONResult(result jsonResult) {
	printJSONLine(result)
}

// printJSONLine prints v as a single line of JSON, exiting if it won't marshal.
func printJSONLine(v any) {
	body, err := json.Marshal(v)
	if err != nil {
		fatal("rendering JSON result: %v", err)
	}
	fmt.Println(string(body))
}
