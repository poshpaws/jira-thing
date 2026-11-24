package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/ops"
	"jira-thing/internal/tui"
)

// exploreServer fakes epics, sprints, their issues, and a single issue fetch.
func exploreServer(t *testing.T) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		jql, _ := body["jql"].(string)
		switch {
		case strings.Contains(jql, "issuetype = Epic"):
			json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{
				{"key": "P-1", "fields": map[string]any{"summary": "Payments", "status": map[string]any{"name": "In Progress"}}},
			}})
		case strings.HasSuffix(r.URL.Path, "/sprint"):
			json.NewEncoder(w).Encode(map[string]any{"isLast": true, "values": []map[string]any{
				{"id": 10, "name": "Sprint 10", "state": "future"}, {"id": 9, "name": "Sprint 9", "state": "active"},
			}})
		case strings.HasSuffix(r.URL.Path, "/sprint/9/issue"), jql != "":
			json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{{"key": "P-2", "fields": map[string]any{"summary": "Child"}}}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"key": "P-2", "fields": map[string]any{"summary": "Child"}})
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
}

func TestEpicExplorer(t *testing.T) {
	exploreServer(t)
	ex, err := epicExplorer(mustConnect(), "PROJ")
	if err != nil || ex.title != "Epics in PROJ" || len(ex.groups) != 1 || ex.groups[0] != (tui.ExplorerGroup{ID: "P-1", Title: "P-1 Payments", Detail: "In Progress"}) {
		t.Fatalf("ex=%+v err=%v", ex, err)
	}
	tickets, err := ex.load(ex.groups[0])
	if err != nil || len(tickets) != 1 || tickets[0].Key != "P-2" {
		t.Errorf("tickets=%+v err=%v", tickets, err)
	}
	if _, err := epicExplorer(mustConnect(), ""); err == nil {
		t.Error("expected an error without a project")
	}
}

func TestSprintExplorer_StartsOnActive(t *testing.T) {
	exploreServer(t)
	ex, err := sprintExplorer(mustConnect(), ops.SprintScope{BoardID: 4}, false)
	if err != nil || ex.start != 1 || len(ex.groups) != 2 || ex.groups[1].ID != "9" || ex.title != "Sprints on board 4" {
		t.Fatalf("ex=%+v err=%v", ex, err)
	}
	tickets, err := ex.load(ex.groups[1])
	if err != nil || len(tickets) != 1 {
		t.Errorf("tickets=%+v err=%v", tickets, err)
	}
}

func TestRunExplorer_ActsThenReopensOnSameGroup(t *testing.T) {
	exploreServer(t)
	oldShow, oldPause := showExplorerFn, pauseFn
	defer func() { showExplorerFn, pauseFn = oldShow, oldPause }()
	var starts []int
	results := []tui.ExplorerResult{{Action: tui.ActionView, Key: "P-2", Group: 3}, {}}
	showExplorerFn = func(_ string, _ []tui.ExplorerGroup, _ tui.GroupLoader, start int) (tui.ExplorerResult, error) {
		starts = append(starts, start)
		res := results[0]
		results = results[1:]
		return res, nil
	}
	pauses := 0
	pauseFn = func() { pauses++ }
	out := captureStdout(func() { runExplorer(mustConnect(), explorer{title: "x", start: 1}) })
	if len(starts) != 2 || starts[0] != 1 || starts[1] != 3 || pauses != 1 || !strings.Contains(out, "P-2") {
		t.Errorf("starts=%v pauses=%d out=%s", starts, pauses, out)
	}
}

func TestExplore_Usage(t *testing.T) {
	for _, args := range [][]string{{"explore", "extra"}} {
		captureStderr(func() {
			if !captureExit(func() { runEpic(args) }) {
				t.Errorf("epic %v: expected exit", args)
			}
			if !captureExit(func() { runSprint(args) }) {
				t.Errorf("sprint %v: expected exit", args)
			}
		})
	}
}
