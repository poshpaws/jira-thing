package main

import (
	"encoding/json"
	"fmt"
	"os"

	"jira-thing/internal/api"
	"jira-thing/internal/tui"
)

// runBatchCreate reads a JSON array of ticket templates from path and creates
// each one non-interactively, continuing past per-ticket failures and
// reporting how many of the total succeeded. With dryRun, resolves and prints
// each ticket's payload instead of creating anything.
func runBatchCreate(path string, dryRun bool) {
	tickets := loadBatchTickets(path)
	conn := mustConnect()
	if dryRun {
		previewBatchTickets(conn, tickets)
		return
	}

	fmt.Printf("Creating %d ticket(s) from %s...\n", len(tickets), path)
	created := 0
	for i, tmpl := range tickets {
		key, err := createBatchTicket(conn, tmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
			continue
		}
		fmt.Printf("  %s %s  %s\n", tui.SuccessStyle.Render("Created:"), tui.KeyStyle.Render(key), getString(tmpl, "summary"))
		created++
	}
	fmt.Printf("\n%s %d of %d ticket(s) created\n", tui.SuccessStyle.Render("Done:"), created, len(tickets))
}

// loadBatchTickets reads and parses path as a non-empty JSON array of ticket templates.
func loadBatchTickets(path string) []map[string]any {
	data, err := os.ReadFile(path) // #nosec G304 -- user-supplied CLI input
	if err != nil {
		fatal("reading batch file: %v", err)
	}
	var tickets []map[string]any
	if err := json.Unmarshal(data, &tickets); err != nil {
		fatal("parsing batch file as a JSON array: %v", err)
	}
	if len(tickets) == 0 {
		fatal("no tickets found in %s", path)
	}
	return tickets
}

// previewBatchTickets resolves and prints each batch entry's payload without
// creating anything, continuing past per-ticket resolution failures just like
// a real batch create would continue past per-ticket API failures.
func previewBatchTickets(conn api.JiraConnection, tickets []map[string]any) {
	fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Dry run — %d ticket(s), none will be created:", len(tickets))))
	for i, tmpl := range tickets {
		if err := resolveCreateFields(tmpl, "", true); err != nil {
			fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
			continue
		}
		payload, parentKey, err := prepareTicketPayload(conn, tmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
			continue
		}
		fmt.Printf("\n[%d/%d]\n", i+1, len(tickets))
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint, payload)
		printParentLinkStep(parentKey)
	}
}

// createBatchTicket resolves fields non-interactively and creates a single
// ticket from a batch entry.
func createBatchTicket(conn api.JiraConnection, tmpl map[string]any) (string, error) {
	if err := resolveCreateFields(tmpl, "", true); err != nil {
		return "", err
	}
	return createTicket(conn, tmpl)
}
