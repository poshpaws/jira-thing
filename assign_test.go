package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/tui"
)

// assignHandler answers user search with users and records assignee PUT bodies.
func assignHandler(users []map[string]any, puts *[]map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/PROJ-1/assignee") {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			*puts = append(*puts, body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(users)
	}
}

func cliUser(id, name string) map[string]any {
	return map[string]any{"accountId": id, "displayName": name, "active": true, "accountType": "atlassian"}
}

// withInteractive makes runAssign think it is (or isn't) attached to a terminal.
func withInteractive(interactive bool) func() {
	old := isInteractiveFn
	isInteractiveFn = func() bool { return interactive }
	return func() { isInteractiveFn = old }
}

func startAssign(t *testing.T, users []map[string]any) *[]map[string]any {
	t.Helper()
	puts := &[]map[string]any{}
	srv := httptest.NewTLSServer(assignHandler(users, puts))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	return puts
}

func TestRunAssign_ByName(t *testing.T) {
	puts := startAssign(t, []map[string]any{cliUser("a1", "Ada Lovelace")})
	out := captureStdout(func() { runAssign([]string{"PROJ-1", "ada"}) })
	if len(*puts) != 1 || (*puts)[0]["accountId"] != "a1" || !strings.Contains(out, "Ada Lovelace (a1)") {
		t.Errorf("puts=%v out=%s", *puts, out)
	}
}

func TestRunAssign_JSONAndDryRun(t *testing.T) {
	puts := startAssign(t, []map[string]any{cliUser("a1", "Ada")})
	out := captureStdout(func() { runAssign([]string{"PROJ-1", "ada", "-dry-run", "-json"}) })
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %s", out)
	}
	if got["status"] != "dry-run" || got["accountId"] != "a1" || len(*puts) != 0 {
		t.Errorf("got=%v puts=%v", got, *puts)
	}
}

func TestRunAssign_AmbiguousNonInteractiveListsCandidates(t *testing.T) {
	defer withInteractive(false)()
	puts := startAssign(t, []map[string]any{cliUser("a1", "Ada One"), cliUser("a2", "Ada Two")})
	errOut := captureStderr(func() {
		if !captureExit(func() { runAssign([]string{"PROJ-1", "ada"}) }) {
			t.Error("expected exit on ambiguous name")
		}
	})
	if !strings.Contains(errOut, "Ada One (a1)") || !strings.Contains(errOut, "Ada Two (a2)") || len(*puts) != 0 {
		t.Errorf("puts=%v stderr=%s", *puts, errOut)
	}
}

func TestRunAssign_AmbiguousInteractiveUsesPick(t *testing.T) {
	defer withInteractive(true)()
	old := selectMenuOptionFn
	defer func() { selectMenuOptionFn = old }()
	var title string
	selectMenuOptionFn = func(t string, options []tui.MenuOption) (int, bool, error) {
		title = t
		return 1, false, nil
	}
	puts := startAssign(t, []map[string]any{cliUser("a1", "Ada One"), cliUser("a2", "Ada Two")})
	captureStdout(func() { runAssign([]string{"PROJ-1", "ada"}) })
	if title != "Pick an assignee" || len(*puts) != 1 || (*puts)[0]["accountId"] != "a2" {
		t.Errorf("title=%q puts=%v", title, *puts)
	}
}

func TestRunAssign_PickerCancelled(t *testing.T) {
	defer withInteractive(true)()
	old := selectMenuOptionFn
	defer func() { selectMenuOptionFn = old }()
	selectMenuOptionFn = func(string, []tui.MenuOption) (int, bool, error) { return 0, true, nil }
	puts := startAssign(t, []map[string]any{cliUser("a1", "Ada One"), cliUser("a2", "Ada Two")})
	errOut := captureStderr(func() { captureExit(func() { runAssign([]string{"PROJ-1", "ada"}) }) })
	if !strings.Contains(errOut, "cancelled") || len(*puts) != 0 {
		t.Errorf("puts=%v stderr=%s", *puts, errOut)
	}
}
