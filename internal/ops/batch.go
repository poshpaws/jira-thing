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

// BatchOptions controls a batch create. With Dedupe, entries duplicating an
// open ticket or an earlier entry are skipped; with DryRun, nothing is
// created. Defaults fill each entry's unset fields. An entry's "sprint" is
// looked up on Scope(its project), or the project's only sprint board when
// Scope is nil; its "points" go to the first of PointFields on the edit screen.
type BatchOptions struct {
	Dedupe, DryRun bool
	Defaults       CreateDefaults
	Scope          func(project string) SprintScope
	PointFields    []string
}

// CreateTicketsBatch creates every entry non-interactively, continuing past
// per-entry failures.
func CreateTicketsBatch(conn api.JiraConnection, entries []map[string]any, batch BatchOptions) []BatchItem {
	guard := NewDedupeGuard(conn, batch.Dedupe)
	items := make([]BatchItem, len(entries))
	for i, entry := range entries {
		opts := CreateOptions{Dedupe: guard, DryRun: batch.DryRun, Label: fmt.Sprintf("ticket %d in this batch", i+1)}
		res, err := createBatchEntry(conn, entry, batch, opts)
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
