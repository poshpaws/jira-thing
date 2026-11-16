package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/ops"
)

// lifecycleCLI fakes issue fetch, create, delete, worklog and myself,
// recording each mutating request as "METHOD path?query".
type lifecycleCLI struct {
	fields map[string]any
	email  string
	sent   []string
	bodies []map[string]any
}

func (j *lifecycleCLI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		j.sent = append(j.sent, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		j.bodies = append(j.bodies, body)
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/myself"):
		json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me", "displayName": "Me", "emailAddress": j.email})
	case r.Method == http.MethodGet:
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1", "fields": j.fields})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issue"):
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-9"})
	case strings.HasSuffix(r.URL.Path, "/worklog"):
		json.NewEncoder(w).Encode(map[string]any{"id": "1"})
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func startLifecycleCLI(t *testing.T, j *lifecycleCLI) {
	t.Helper()
	srv := httptest.NewTLSServer(j)
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
}

func cloneSource() map[string]any {
	return map[string]any{
		"project": map[string]any{"key": "PROJ"}, "issuetype": map[string]any{"name": "Task"},
		"summary": "Patch foo", "labels": []any{"old"},
	}
}

func TestRunWorklogAdd(t *testing.T) {
	j := &lifecycleCLI{}
	startLifecycleCLI(t, j)
	out := captureStdout(func() { runWorklog([]string{"add", "PROJ-1", "2h", "30m", "-comment", "pairing"}) })
	if len(j.bodies) != 1 || j.bodies[0]["timeSpent"] != "2h 30m" || !strings.Contains(out, "Logged 2h 30m on") {
		t.Errorf("bodies=%v out=%s", j.bodies, out)
	}
}

func TestRunWorklogAdd_DryRunAndJSON(t *testing.T) {
	j := &lifecycleCLI{}
	startLifecycleCLI(t, j)
	out := captureStdout(func() { runWorklog([]string{"add", "PROJ-1", "1d", "-dry-run"}) })
	if !strings.Contains(out, "/rest/api/3/issue/PROJ-1/worklog") || len(j.sent) != 0 {
		t.Errorf("sent=%v out=%s", j.sent, out)
	}
	out = captureStdout(func() { runWorklog([]string{"add", "PROJ-1", "45m", "-json"}) })
	var got worklogJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.TimeSpent != "45m" || got.Status != "logged" {
		t.Errorf("got=%+v out=%s", got, out)
	}
}

func TestRunWorklogAdd_BadDuration(t *testing.T) {
	startLifecycleCLI(t, &lifecycleCLI{})
	errOut := captureStderr(func() {
		if !captureExit(func() { runWorklog([]string{"add", "PROJ-1", "two", "hours"}) }) {
			t.Error("expected exit")
		}
	})
	if !strings.Contains(errOut, "Jira's format") {
		t.Errorf("stderr = %s", errOut)
	}
}

func TestRunDelete(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		interactive bool
		stdin       string
		wantSent    string
		wantOut     string
		wantExit    bool
	}{
		{"yes flag", []string{"PROJ-1", "-y"}, false, "", "DELETE /rest/api/3/issue/PROJ-1?deleteSubtasks=false", "Deleted", false},
		{"confirmed", []string{"PROJ-1"}, true, "y", "DELETE", "Deleted", false},
		{"declined", []string{"PROJ-1"}, true, "n", "", "", true},
		{"no terminal", []string{"PROJ-1"}, false, "", "", "", true},
		{"dry run", []string{"PROJ-1", "-dry-run"}, false, "", "", "Dry run: would delete", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &lifecycleCLI{fields: map[string]any{"summary": "Old"}}
			startLifecycleCLI(t, j)
			defer withInteractive(tt.interactive)()
			defer pipeStdinLines(t, tt.stdin)()
			var exited bool
			out := captureStdout(func() { captureStderr(func() { exited = captureExit(func() { runDelete(tt.args) }) }) })
			sent := strings.Join(j.sent, " ")
			if exited != tt.wantExit || !strings.Contains(sent, tt.wantSent) || tt.wantSent == "" && sent != "" || !strings.Contains(out, tt.wantOut) {
				t.Errorf("exited=%v sent=%s out=%s", exited, sent, out)
			}
		})
	}
}

func TestRunDelete_SubtasksNeedCascade(t *testing.T) {
	j := &lifecycleCLI{fields: map[string]any{"summary": "Parent", "subtasks": []any{map[string]any{"key": "PROJ-2"}}}}
	startLifecycleCLI(t, j)
	errOut := captureStderr(func() { captureExit(func() { runDelete([]string{"PROJ-1", "-y"}) }) })
	if !strings.Contains(errOut, "PROJ-2") || len(j.sent) != 0 {
		t.Errorf("sent=%v stderr=%s", j.sent, errOut)
	}
	out := captureStdout(func() { runDelete([]string{"PROJ-1", "-y", "-cascade", "-json"}) })
	if !strings.Contains(out, `"subtasks":["PROJ-2"]`) || !strings.Contains(strings.Join(j.sent, " "), "deleteSubtasks=true") {
		t.Errorf("sent=%v out=%s", j.sent, out)
	}
}

func TestRunClone(t *testing.T) {
	j := &lifecycleCLI{fields: cloneSource()}
	startLifecycleCLI(t, j)
	out := captureStdout(func() { runClone([]string{"PROJ-1", "-replace", "foo:bar", "-labels", "new"}) })
	fields, _ := j.bodies[0]["fields"].(map[string]any)
	if fields["summary"] != "CLONE - Patch bar" || !strings.Contains(out, "PROJ-9") {
		t.Errorf("fields=%v out=%s", fields, out)
	}
}

func TestRunClone_DryRunAndBadReplace(t *testing.T) {
	j := &lifecycleCLI{fields: cloneSource()}
	startLifecycleCLI(t, j)
	out := captureStdout(func() { runClone([]string{"PROJ-1", "-summary", "Fresh", "-dry-run"}) })
	if !strings.Contains(out, `"summary": "Fresh"`) || len(j.sent) != 0 {
		t.Errorf("sent=%v out=%s", j.sent, out)
	}
	errOut := captureStderr(func() {
		if !captureExit(func() { runClone([]string{"PROJ-1", "-replace", "nocolon"}) }) {
			t.Error("expected exit")
		}
	})
	if !strings.Contains(errOut, "find:replace") {
		t.Errorf("stderr = %s", errOut)
	}
}

func TestRunMe(t *testing.T) {
	j := &lifecycleCLI{email: "me@x.io"}
	startLifecycleCLI(t, j)
	if out := captureStdout(func() { runMe(nil) }); strings.TrimSpace(out) != "me@x.io" {
		t.Errorf("out = %q", out)
	}
	j.email = ""
	if out := captureStdout(func() { runMe(nil) }); strings.TrimSpace(out) != "acc-me" {
		t.Errorf("hidden email: out = %q", out)
	}
	if out := captureStdout(func() { runMe([]string{"-json"}) }); !strings.Contains(out, `"accountId":"acc-me"`) {
		t.Errorf("json: out = %s", out)
	}
}

func TestLifecycle_Usage(t *testing.T) {
	cases := []func(){
		func() { runWorklog(nil) }, func() { runWorklog([]string{"bogus"}) }, func() { runWorklog([]string{"add", "PROJ-1"}) },
		func() { runDelete(nil) }, func() { runClone(nil) }, func() { runMe([]string{"extra"}) },
	}
	for i, c := range cases {
		captureStderr(func() {
			if !captureExit(c) {
				t.Errorf("case %d: expected exit", i)
			}
		})
	}
}

func TestDeleteDescription_CantForgeLines(t *testing.T) {
	got := deleteDescription(ops.DeleteResult{Key: "PROJ-1", Summary: "Old\nSubtasks: none\x1b[2J", Subtasks: []string{"PROJ-2\nPROJ-3"}})
	if strings.Count(got, "\n") != 1 || strings.Contains(got, "\x1b[2J") {
		t.Errorf("description = %q", got)
	}
}
