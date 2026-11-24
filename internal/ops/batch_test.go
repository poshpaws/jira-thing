package ops

import (
	"strings"
	"testing"
)

func TestCreateTicketsBatch_ContinuesPastFailuresAndTallies(t *testing.T) {
	f := &fakeJira{existing: map[string]string{"Dup": "PROJ-7"}}
	conn := startFake(t, f)
	items := CreateTicketsBatch(conn, []map[string]any{
		{"project": "PROJ", "summary": "One", "description": "**md**"},
		{"project": "PROJ"},
		{"project": "PROJ", "summary": "dup"},
	}, BatchOptions{Dedupe: true})
	tally := Tally(items)
	if tally != (BatchTally{Created: 1, Skipped: 1, Failed: 1}) {
		t.Errorf("tally=%+v items=%+v", tally, items)
	}
	if desc, ok := f.created[0]["description"].(map[string]any); !ok || desc["type"] != "doc" {
		t.Errorf("markdown description not converted to ADF: %v", f.created[0]["description"])
	}
	if items[1].Err == nil || !strings.Contains(items[1].Err.Error(), "summary") {
		t.Errorf("expected missing-summary error, got %v", items[1].Err)
	}
}

func TestCreateTicketsBatch_DryRunFlagsRepeats(t *testing.T) {
	f := &fakeJira{}
	conn := startFake(t, f)
	items := CreateTicketsBatch(conn, []map[string]any{
		{"project": "PROJ", "summary": "Same"}, {"project": "PROJ", "summary": "same"},
	}, BatchOptions{Dedupe: true, DryRun: true})
	if items[1].Result.Key != "ticket 1 in this batch" || len(f.created) != 0 {
		t.Errorf("items=%+v created=%d", items, len(f.created))
	}
}

func TestSetTemplateAssignee(t *testing.T) {
	withAssignee := func() map[string]any { return map[string]any{"assignee": map[string]any{"accountId": "a1"}} }
	kept := withAssignee()
	SetTemplateAssignee(kept, true)
	replaced := withAssignee()
	SetTemplateAssignee(replaced, false)
	none := map[string]any{}
	SetTemplateAssignee(none, true)
	if nestedStr(kept, "assignee", "accountId") != "a1" || replaced["assignee"] != "__SELF__" || none["assignee"] != "__SELF__" {
		t.Errorf("kept=%v replaced=%v none=%v", kept, replaced, none)
	}
}
