package ops

import (
	"errors"
	"strings"
	"testing"

	"jira-thing/internal/config"
)

func TestCreateTicketsBatch_SprintAndPointsPerEntry(t *testing.T) {
	f := &fakeJira{editFields: []string{"customfield_10016"}}
	var scopedFor []string
	batch := BatchOptions{Scope: func(project string) SprintScope {
		scopedFor = append(scopedFor, project)
		return SprintScope{Project: project}
	}}
	entries := []map[string]any{
		{"project": "PROJ", "summary": "One", "sprint": "42", "points": 3.0},
		{"project": "PROJ", "summary": "Two", "sprint": 43.0},
		{"project": "PROJ", "summary": "Three"},
	}
	items := CreateTicketsBatch(startFake(t, f), entries, batch)
	if Tally(items).Created != 3 {
		t.Fatalf("items = %+v", items)
	}
	if len(f.sprintAdds) != 2 || !strings.HasSuffix(f.sprintAdds[0], "/sprint/42/issue") || !strings.HasSuffix(f.sprintAdds[1], "/sprint/43/issue") {
		t.Errorf("sprint posts = %v", f.sprintAdds)
	}
	if len(f.updates) != 1 || f.updates[0]["customfield_10016"] != 3.0 {
		t.Errorf("updates = %v", f.updates)
	}
	for _, created := range f.created {
		if _, leaked := created["sprint"]; leaked {
			t.Errorf("sprint must not reach the POST: %v", created)
		}
		if _, leaked := created["points"]; leaked {
			t.Errorf("points must not reach the POST: %v", created)
		}
	}
	if len(scopedFor) != 2 || scopedFor[0] != "PROJ" {
		t.Errorf("scope asked for %v", scopedFor)
	}
}

func TestCreateTicketsBatch_BadSprintOrPointsFailOnlyThatEntry(t *testing.T) {
	f := &fakeJira{}
	entries := []map[string]any{
		{"project": "PROJ", "summary": "Bad points", "points": "lots"},
		{"project": "PROJ", "summary": "Closed sprint", "sprint": "prev"},
		{"project": "PROJ", "summary": "Bad sprint type", "sprint": true},
		{"project": "PROJ", "summary": "Fine"},
	}
	items := CreateTicketsBatch(startFake(t, f), entries, BatchOptions{})
	if !errors.Is(items[0].Err, ErrBadStoryPoints) || !errors.Is(items[1].Err, ErrClosedSprintTarget) || !errors.Is(items[2].Err, ErrBadSprint) {
		t.Errorf("errs = %v / %v / %v", items[0].Err, items[1].Err, items[2].Err)
	}
	if len(f.created) != 1 || items[3].Err != nil {
		t.Errorf("only the fine entry should be created: created=%d items=%+v", len(f.created), items)
	}
}

func TestConfigBatchOptions(t *testing.T) {
	got := ConfigBatchOptions(config.Config{Project: "PROJ", Board: 9, DefaultPriority: "High", StoryPointsField: "customfield_1"})
	if got.Defaults.Priority != "High" || got.PointFields[0] != "customfield_1" || got.Scope("PROJ").BoardID != 9 || got.Scope("OTHER").BoardID != 0 {
		t.Errorf("got %+v", got)
	}
}
