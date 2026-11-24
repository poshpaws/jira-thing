package ops

import (
	"testing"

	"jira-thing/internal/config"
)

func TestCreateTicket_DefaultsFillOnlyMissingFields(t *testing.T) {
	f := &fakeJira{}
	defaults := CreateDefaults{Priority: "High", Components: []string{"Backend"}, Labels: []string{"from-config"}}
	req := CreateRequest{
		Fields:   map[string]any{"project": "PROJ", "summary": "S", "labels": []any{"from-template"}},
		Defaults: defaults,
	}
	if _, err := CreateTicket(startFake(t, f), req, CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	got := f.created[0]
	if nestedStr(got, "priority", "name") != "High" {
		t.Errorf("priority = %v", got["priority"])
	}
	comps, _ := got["components"].([]any)
	if len(comps) != 1 || str(comps[0].(map[string]any), "name") != "Backend" {
		t.Errorf("components = %v", got["components"])
	}
	labels, _ := got["labels"].([]any)
	if len(labels) != 1 || labels[0] != "from-template" {
		t.Errorf("template labels must win over defaults: %v", got["labels"])
	}
}

func TestCreateTicket_TypedFieldsWinOverDefaults(t *testing.T) {
	f := &fakeJira{}
	req := CreateRequest{Project: "PROJ", Summary: "S", Priority: "Low", Defaults: CreateDefaults{Priority: "High"}}
	if _, err := CreateTicket(startFake(t, f), req, CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if nestedStr(f.created[0], "priority", "name") != "Low" {
		t.Errorf("priority = %v", f.created[0]["priority"])
	}
}

func TestCreateTicketsBatch_AppliesDefaults(t *testing.T) {
	f := &fakeJira{}
	entries := []map[string]any{{"project": "PROJ", "summary": "One"}, {"project": "PROJ", "summary": "Two", "priority": "Low"}}
	items := CreateTicketsBatch(startFake(t, f), entries, BatchOptions{Defaults: CreateDefaults{Priority: "High"}})
	if Tally(items).Created != 2 {
		t.Fatalf("items = %+v", items)
	}
	if nestedStr(f.created[0], "priority", "name") != "High" || nestedStr(f.created[1], "priority", "name") != "Low" {
		t.Errorf("created = %v", f.created)
	}
}

func TestDefaultsFromConfig(t *testing.T) {
	cfg := config.Config{DefaultPriority: "High", DefaultComponents: []string{"A"}, DefaultLabels: []string{"l"}}
	got := DefaultsFromConfig(cfg)
	if got.Priority != "High" || len(got.Components) != 1 || len(got.Labels) != 1 {
		t.Errorf("got %+v", got)
	}
}
