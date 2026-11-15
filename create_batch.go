package main

import (
	"encoding/json"
	"fmt"
	"os"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
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
	if dryRun {
		fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Dry run — %d ticket(s), none will be created:", len(tickets))))
	} else {
		fmt.Printf("Creating %d ticket(s) from %s...\n", len(tickets), path)
	}
	items := ops.CreateTicketsBatch(conn, tickets, dedupe, dryRun)
	for _, it := range items {
		printBatchItem(conn, it, len(items))
	}
	if !dryRun {
		tally := ops.Tally(items)
		fmt.Printf("\n%s %s\n", tui.SuccessStyle.Render("Done:"), batchSummary(tally.Created, tally.Skipped, len(items)))
	}
}

// printBatchItem reports one entry: its error, its skip as a duplicate, its
// dry-run payload, or the ticket created.
func printBatchItem(conn api.JiraConnection, it ops.BatchItem, total int) {
	switch {
	case it.Err != nil:
		printBatchError(it.Index, it.Err)
	case it.Result.Status == ops.StatusExists:
		printBatchSkipped(it.Result.Key, it.Summary)
	case it.Result.Status == ops.StatusDryRun:
		fmt.Printf("\n[%d/%d]\n", it.Index+1, total)
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint, it.Result.Payload)
		printParentLinkStep(it.Result.Parent)
	default:
		fmt.Printf("  %s %s  %s\n", tui.SuccessStyle.Render("Created:"), tui.KeyStyle.Render(it.Result.Key), it.Summary)
	}
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
func printBatchSkipped(existing, summary string) {
	fmt.Printf("  %s %s  %s (duplicate)\n", tui.SuccessStyle.Render("Skipped:"), tui.KeyStyle.Render(existing), termsafe.Clean(summary))
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
