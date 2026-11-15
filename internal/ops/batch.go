package ops

import (
	"fmt"

	"jira-thing/internal/api"
)

// BatchItem reports one batch entry: its result, or the error that stopped it.
type BatchItem struct {
	Index   int
	Summary string
	Result  CreateResult
	Err     error
}

// BatchTally counts a batch's outcomes.
type BatchTally struct {
	Created, Skipped, Failed int
}

// CreateTicketsBatch creates every entry non-interactively, continuing past
// per-entry failures. With dedupe, entries duplicating an open ticket or an
// earlier entry are skipped; with dryRun, nothing is created.
func CreateTicketsBatch(conn api.JiraConnection, entries []map[string]any, dedupe, dryRun bool) []BatchItem {
	guard := NewDedupeGuard(conn, dedupe)
	items := make([]BatchItem, len(entries))
	for i, entry := range entries {
		opts := CreateOptions{Dedupe: guard, DryRun: dryRun, Label: fmt.Sprintf("ticket %d in this batch", i+1)}
		res, err := CreateTicket(conn, CreateRequest{Fields: entry}, opts)
		if err == nil {
			err = ParentLinkError(res)
		}
		items[i] = BatchItem{Index: i, Summary: str(entry, "summary"), Result: res, Err: err}
	}
	return items
}

// Tally counts the outcomes in items.
func Tally(items []BatchItem) BatchTally {
	var t BatchTally
	for _, it := range items {
		switch {
		case it.Err != nil:
			t.Failed++
		case it.Result.Status == StatusExists:
			t.Skipped++
		default:
			t.Created++
		}
	}
	return t
}

// ParentLinkError reports a ticket that was created but not linked to its parent.
func ParentLinkError(res CreateResult) error {
	if res.ParentErr == nil {
		return nil
	}
	return fmt.Errorf("created %s but failed to link to parent %s: %w", res.Key, res.Parent, res.ParentErr)
}
