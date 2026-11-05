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
// each ticket's payload instead of creating anything. With dedupe, entries
// duplicating an open ticket, or an earlier entry, are skipped.
func runBatchCreate(path string, dryRun, dedupe bool) {
	tickets := loadBatchTickets(path)
	conn := mustConnect()
	guard := newDedupeGuard(conn, dedupe)
	if dryRun {
		previewBatchTickets(conn, guard, tickets)
		return
	}

	fmt.Printf("Creating %d ticket(s) from %s...\n", len(tickets), path)
	created, skipped := 0, 0
	for i, tmpl := range tickets {
		key, duplicate, err := createBatchTicket(conn, guard, tmpl)
		switch {
		case err != nil:
			printBatchError(i, err)
		case duplicate:
			printBatchSkipped(key, tmpl)
			skipped++
		default:
			fmt.Printf("  %s %s  %s\n", tui.SuccessStyle.Render("Created:"), tui.KeyStyle.Render(key), getString(tmpl, "summary"))
			created++
		}
	}
	fmt.Printf("\n%s %s\n", tui.SuccessStyle.Render("Done:"), batchSummary(created, skipped, len(tickets)))
}

// batchSummary words the closing tally, mentioning skips only if there were any.
func batchSummary(created, skipped, total int) string {
	tally := fmt.Sprintf("%d of %d ticket(s) created", created, total)
	if skipped > 0 {
		tally += fmt.Sprintf(", %d skipped as duplicates", skipped)
	}
	return tally
}

// printBatchError reports the failure of the ticket at index i.
func printBatchError(i int, err error) {
	fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
}

// printBatchSkipped reports an entry skipped as a duplicate of existing.
func printBatchSkipped(existing string, tmpl map[string]any) {
	fmt.Printf("  %s %s  %s (duplicate)\n", tui.SuccessStyle.Render("Skipped:"), tui.KeyStyle.Render(existing), getString(tmpl, "summary"))
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
func previewBatchTickets(conn api.JiraConnection, guard *dedupeGuard, tickets []map[string]any) {
	fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Dry run — %d ticket(s), none will be created:", len(tickets))))
	for i, tmpl := range tickets {
		if err := previewBatchTicket(conn, guard, tmpl, i, len(tickets)); err != nil {
			printBatchError(i, err)
		}
	}
}

// previewBatchTicket prints what a real run would do with the entry at index i.
func previewBatchTicket(conn api.JiraConnection, guard *dedupeGuard, tmpl map[string]any, i, total int) error {
	if err := resolveCreateFields(tmpl, "", true); err != nil {
		return err
	}
	existing, err := guard.find(tmpl)
	if err != nil {
		return err
	}
	if existing != "" {
		printBatchSkipped(existing, tmpl)
		return nil
	}
	guard.remember(tmpl, fmt.Sprintf("ticket %d in this batch", i+1))
	payload, parentKey, err := prepareTicketPayload(conn, tmpl)
	if err != nil {
		return err
	}
	fmt.Printf("\n[%d/%d]\n", i+1, total)
	printDryRun("POST", conn.BaseURL+api.IssueEndpoint, payload)
	printParentLinkStep(parentKey)
	return nil
}

// createBatchTicket resolves fields non-interactively and creates a single
// ticket from a batch entry. If the entry duplicates an existing ticket, it
// returns that ticket's key with duplicate set, and creates nothing.
func createBatchTicket(conn api.JiraConnection, guard *dedupeGuard, tmpl map[string]any) (key string, duplicate bool, err error) {
	if err := resolveCreateFields(tmpl, "", true); err != nil {
		return "", false, err
	}
	existing, err := guard.find(tmpl)
	if err != nil {
		return "", false, err
	}
	if existing != "" {
		return existing, true, nil
	}
	key, err = createTicket(conn, tmpl)
	if key != "" {
		guard.remember(tmpl, key)
	}
	return key, false, err
}
