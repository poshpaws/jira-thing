package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-thing/internal/config"
)

// sprintCLIHandler fakes boards, sprints and sprint issues, recording POSTed keys.
func sprintCLIHandler(posted *[][]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			var body struct{ Issues []string }
			json.NewDecoder(r.Body).Decode(&body)
			*posted = append(*posted, body.Issues)
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/sprint"):
			json.NewEncoder(w).Encode(map[string]any{"isLast": true, "values": []map[string]any{
				{"id": 11, "name": "Sprint 11", "state": "active", "startDate": "2026-11-02T09:00:00Z", "endDate": "2026-11-16T09:00:00Z"},
			}})
		case strings.HasSuffix(r.URL.Path, "/issue"):
			json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{
				{"key": "PROJ-1", "fields": map[string]any{"summary": "First"}},
			}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"values": []map[string]any{{"id": 4, "name": "Team", "type": "scrum"}}})
		}
	}
}

// startSprintCLI serves the fake Agile API with "project": "PROJ" in config.
func startSprintCLI(t *testing.T) *[][]string {
	t.Helper()
	posted := &[][]string{}
	srv := httptest.NewTLSServer(sprintCLIHandler(posted))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	path := filepath.Join(t.TempDir(), "jira-thing.json")
	if err := os.WriteFile(path, []byte(`{"project":"PROJ"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	t.Cleanup(func() { config.ConfigPath = old })
	return posted
}

func TestRunSprintList_Sprints(t *testing.T) {
	startSprintCLI(t)
	out := captureStdout(func() { runSprint([]string{"list"}) })
	if !strings.Contains(out, "Board 4: Team") || !strings.Contains(out, "Sprint 11") || !strings.Contains(out, "2026-11-02 → 2026-11-16") {
		t.Errorf("out = %s", out)
	}
}

func TestRunSprintList_JSON(t *testing.T) {
	startSprintCLI(t)
	out := captureStdout(func() { runSprint([]string{"list", "-json"}) })
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 1 || got[0]["name"] != "Sprint 11" {
		t.Errorf("got=%v err=%v out=%s", got, err, out)
	}
}

func TestRunSprintList_CurrentIssues(t *testing.T) {
	startSprintCLI(t)
	out := captureStdout(func() { runSprint([]string{"list", "-current"}) })
	if !strings.Contains(out, "Sprint 11 (11)") || !strings.Contains(out, "PROJ-1") {
		t.Errorf("out = %s", out)
	}
}

func TestRunSprintList_IssuesJSON(t *testing.T) {
	startSprintCLI(t)
	out := captureStdout(func() { runSprint([]string{"list", "-sprint", "11", "-json"}) })
	if !strings.Contains(out, `"key": "PROJ-1"`) {
		t.Errorf("out = %s", out)
	}
}

func TestRunSprintList_TwoSelectorsFail(t *testing.T) {
	startSprintCLI(t)
	errOut := captureStderr(func() {
		if !captureExit(func() { runSprint([]string{"list", "-current", "-next"}) }) {
			t.Error("expected exit")
		}
	})
	if !strings.Contains(errOut, "pick one of") {
		t.Errorf("stderr = %s", errOut)
	}
}

func TestRunSprintAdd(t *testing.T) {
	posted := startSprintCLI(t)
	out := captureStdout(func() { runSprint([]string{"add", "current", "PROJ-1", "PROJ-2"}) })
	if len(*posted) != 1 || len((*posted)[0]) != 2 || !strings.Contains(out, "Added 2 ticket(s) to sprint Sprint 11 (11)") {
		t.Errorf("posted=%v out=%s", *posted, out)
	}
}

func TestRunSprintAdd_DryRunJSON(t *testing.T) {
	posted := startSprintCLI(t)
	out := captureStdout(func() { runSprint([]string{"add", "42", "PROJ-1", "-dry-run", "-json"}) })
	var got sprintAddJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Status != "dry-run" || got.SprintID != 42 || len(*posted) != 0 {
		t.Errorf("got=%+v posted=%v out=%s", got, *posted, out)
	}
}

func TestRunSprintAdd_DryRunShowsRequest(t *testing.T) {
	startSprintCLI(t)
	out := captureStdout(func() { runSprint([]string{"add", "42", "PROJ-1", "-dry-run"}) })
	if !strings.Contains(out, "/rest/agile/1.0/sprint/42/issue") || !strings.Contains(out, "PROJ-1") {
		t.Errorf("out = %s", out)
	}
}

func TestRunSprint_Usage(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"add", "current"}} {
		captureStderr(func() {
			if !captureExit(func() { runSprint(args) }) {
				t.Errorf("args %v: expected exit", args)
			}
		})
	}
}

func TestMenuListSprints_ListsProjectBoard(t *testing.T) {
	startSprintCLI(t)
	defer pipeStdinLines(t, "")()
	out := captureStdout(func() { menuListSprints(mustConnect()) })
	if !strings.Contains(out, "Sprint 11") {
		t.Errorf("out = %s", out)
	}
}

func TestMenuSprint_BadBoardID(t *testing.T) {
	startSprintCLI(t)
	defer pipeStdinLines(t, "four")()
	errOut := captureStderr(func() { captureStdout(func() { menuAddToSprint(mustConnect()) }) })
	if !strings.Contains(errOut, "invalid board ID") {
		t.Errorf("stderr = %s", errOut)
	}
}

func TestMenuAddToSprint_BlankCancels(t *testing.T) {
	posted := startSprintCLI(t)
	defer pipeStdinLines(t, "")()
	captureStdout(func() { menuAddToSprint(mustConnect()) })
	if len(*posted) != 0 {
		t.Errorf("posted = %v", *posted)
	}
}
