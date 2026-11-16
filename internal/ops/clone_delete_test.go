package ops

import (
	"errors"
	"strings"
	"testing"
)

func adfDoc(text string) map[string]any {
	return map[string]any{"type": "doc", "version": 1, "content": []any{
		map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}},
	}}
}

func TestCloneTicket_ReplacesAndOverridesLabels(t *testing.T) {
	source := map[string]any{
		"project": map[string]any{"key": "PROJ"}, "issuetype": map[string]any{"name": "Task"},
		"summary": "Patch foo in prod", "labels": []any{"old"}, "description": adfDoc("Upgrade foo on foo-01"),
	}
	j := &lifecycleJira{source: source}
	req := CloneRequest{SourceKey: "SRC-1", Labels: []string{"new"}, Replace: []Replacement{{Find: "foo", Replace: "bar"}}}
	res, err := CloneTicket(startLifecycle(t, j), req, false)
	if err != nil {
		t.Fatal(err)
	}
	fields, _ := j.bodies[0]["fields"].(map[string]any)
	if fields["summary"] != ClonePrefix+"Patch bar in prod" || res.Payload == nil {
		t.Errorf("summary = %v", fields["summary"])
	}
	if labels, _ := fields["labels"].([]any); len(labels) != 1 || labels[0] != "new" {
		t.Errorf("labels = %v", fields["labels"])
	}
	if desc := prettyText(fields["description"]); !strings.Contains(desc, "Upgrade bar on bar-01") {
		t.Errorf("description = %s", desc)
	}
	if prettyText(source["description"]) != prettyText(adfDoc("Upgrade foo on foo-01")) {
		t.Error("source description was modified")
	}
}

// prettyText flattens a value for substring checks.
func prettyText(v any) string {
	var sb strings.Builder
	var walk func(any)
	walk = func(n any) {
		switch x := n.(type) {
		case map[string]any:
			if s, ok := x["text"].(string); ok {
				sb.WriteString(s)
			}
			walk(x["content"])
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(v)
	return sb.String()
}

func TestCloneTicket_DryRunSendsNothing(t *testing.T) {
	j := &lifecycleJira{source: map[string]any{"project": map[string]any{"key": "PROJ"}, "issuetype": map[string]any{"name": "Task"}, "summary": "S"}}
	res, err := CloneTicket(startLifecycle(t, j), CloneRequest{SourceKey: "SRC-1"}, true)
	if err != nil || res.Status != StatusDryRun || len(j.requests) != 0 || res.Payload == nil {
		t.Errorf("res=%+v requests=%v err=%v", res, j.requests, err)
	}
}

func TestParseReplacement(t *testing.T) {
	tests := []struct {
		in      string
		want    Replacement
		wantErr bool
	}{
		{"foo:bar", Replacement{"foo", "bar"}, false},
		{"a:b:c", Replacement{"a", "b:c"}, false},
		{"gone:", Replacement{"gone", ""}, false},
		{"nocolon", Replacement{}, true},
		{":x", Replacement{}, true},
	}
	for _, tt := range tests {
		got, err := ParseReplacement(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseReplacement(%q) = %+v, %v", tt.in, got, err)
		}
	}
}

func TestDeleteTicket(t *testing.T) {
	withSubtasks := map[string]any{"summary": "Parent", "subtasks": []any{map[string]any{"key": "K-2"}}}
	tests := []struct {
		name     string
		source   map[string]any
		req      DeleteRequest
		dryRun   bool
		wantErr  error
		wantSent string
	}{
		{"plain", map[string]any{"summary": "S"}, DeleteRequest{Key: "K-1"}, false, nil, "deleteSubtasks=false"},
		{"cascade", withSubtasks, DeleteRequest{Key: "K-1", Cascade: true}, false, nil, "deleteSubtasks=true"},
		{"subtasks refused", withSubtasks, DeleteRequest{Key: "K-1"}, false, ErrHasSubtasks, ""},
		{"dry run", withSubtasks, DeleteRequest{Key: "K-1", Cascade: true}, true, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &lifecycleJira{source: tt.source}
			res, err := DeleteTicket(startLifecycle(t, j), tt.req, tt.dryRun)
			sent := strings.Join(j.requests, " ")
			if !errors.Is(err, tt.wantErr) || tt.wantSent == "" && sent != "" || !strings.Contains(sent, tt.wantSent) {
				t.Errorf("res=%+v err=%v sent=%s", res, err, sent)
			}
		})
	}
}

func TestDeleteTicket_DryRunListsSubtasks(t *testing.T) {
	j := &lifecycleJira{source: map[string]any{"summary": "Parent", "subtasks": []any{map[string]any{"key": "K-2"}}}}
	res, err := DeleteTicket(startLifecycle(t, j), DeleteRequest{Key: "K-1", Cascade: true}, true)
	if err != nil || res.Status != StatusDryRun || res.Summary != "Parent" || len(res.Subtasks) != 1 {
		t.Errorf("res=%+v err=%v", res, err)
	}
}

func TestLogWork(t *testing.T) {
	j := &lifecycleJira{}
	conn := startLifecycle(t, j)
	if _, err := LogWork(conn, WorklogRequest{Key: "K-1"}, false); !errors.Is(err, ErrTimeSpentRequired) {
		t.Errorf("expected ErrTimeSpentRequired, got %v", err)
	}
	for _, bad := range []string{"2 hours", "h", "2x", "1h; drop"} {
		if _, err := LogWork(conn, WorklogRequest{Key: "K-1", TimeSpent: bad}, false); err == nil {
			t.Errorf("%q: expected a format error", bad)
		}
	}
	dry, err := LogWork(conn, WorklogRequest{Key: "K-1", TimeSpent: "1d 2h 30m"}, true)
	if err != nil || dry.Status != StatusDryRun || len(j.requests) != 0 {
		t.Errorf("dry=%+v requests=%v err=%v", dry, j.requests, err)
	}
	res, err := LogWork(conn, WorklogRequest{Key: "K-1", TimeSpent: "1.5h", Comment: "**did** it"}, false)
	if err != nil || res.Status != StatusLogged || j.bodies[0]["timeSpent"] != "1.5h" || j.bodies[0]["comment"] == nil {
		t.Errorf("res=%+v body=%v err=%v", res, j.bodies, err)
	}
}
