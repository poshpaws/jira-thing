package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/tui"
)

func TestSplitCommaList(t *testing.T) {
	got := splitCommaList(" a, b ,, c")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestInheritedFields_CopiesKnownKeysOnly(t *testing.T) {
	fields := map[string]any{
		"project": map[string]any{"key": "PROJ"},
		"summary": "should not be copied",
	}
	got := inheritedFields(fields)
	if _, ok := got["project"]; !ok {
		t.Errorf("expected project to be inherited")
	}
	if _, ok := got["summary"]; ok {
		t.Errorf("summary should not be inherited")
	}
}

func TestInheritedCloneFields_CopiesKnownKeysOnly(t *testing.T) {
	fields := map[string]any{
		"priority": map[string]any{"name": "High"},
		"assignee": map[string]any{"accountId": "abc"},
	}
	got := inheritedCloneFields(fields)
	if _, ok := got["priority"]; !ok {
		t.Errorf("expected priority to be inherited")
	}
	if _, ok := got["assignee"]; ok {
		t.Errorf("assignee should not be inherited by clone")
	}
}

// mockSelectMenuOption replaces selectMenuOptionFn to pick a fixed index once, then cancel.
func mockSelectMenuOption(pickOnce int) func() {
	old := selectMenuOptionFn
	called := false
	selectMenuOptionFn = func(title string, options []tui.MenuOption) (int, bool, error) {
		if called {
			return 0, true, nil
		}
		called = true
		return pickOnce, false, nil
	}
	return func() { selectMenuOptionFn = old }
}

func TestRunMenu_WhoamiAction(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"accountId": "abc123", "displayName": "Test User", "emailAddress": "t@example.com",
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	actions := menuActions()
	whoamiIdx := -1
	for i, a := range actions {
		if a.label == "Who Am I" {
			whoamiIdx = i
		}
	}
	if whoamiIdx == -1 {
		t.Fatal("Who Am I action not found in menu")
	}
	defer mockSelectMenuOption(whoamiIdx)()

	out := captureStdout(func() { runMenu() })
	if !strings.Contains(out, "Test User") {
		t.Errorf("expected whoami output, got: %s", out)
	}
}

// mockShowTableQuickAction replaces showTableQuickActionsFn to return a fixed result once.
func mockShowTableQuickAction(res tui.TableResult) func() {
	old := showTableQuickActionsFn
	showTableQuickActionsFn = func(tickets []tui.Ticket, fetch tui.TicketFetcher) (tui.TableResult, error) {
		return res, nil
	}
	return func() { showTableQuickActionsFn = old }
}

func TestBrowseWithQuickActions_ViewFetchesAndRenders(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"key": "PROJ-1",
			"fields": map[string]any{
				"summary": "Test ticket",
			},
		})
	}))
	defer srv.Close()
	defer mockShowTableQuickAction(tui.TableResult{Action: tui.ActionView, Key: "PROJ-1"})()

	conn := api.JiraConnection{BaseURL: srv.URL, Email: "a@b.com", APIToken: testAPIToken}
	out := captureStdout(func() { browseWithQuickActions(conn, nil, nil) })
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("expected rendered ticket, got: %s", out)
	}
}

func TestBrowseWithQuickActions_TransitionRunsFlow(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"transitions": []map[string]any{
				{"id": "21", "name": "Done", "to": map[string]any{"id": "5", "name": "Done"}},
			},
		})
	}))
	defer srv.Close()
	defer mockShowTableQuickAction(tui.TableResult{Action: tui.ActionTransition, Key: "PROJ-1"})()
	defer mockSelectTransition("Done")()

	conn := api.JiraConnection{BaseURL: srv.URL, Email: "a@b.com", APIToken: testAPIToken}
	out := captureStdout(func() { browseWithQuickActions(conn, nil, nil) })
	if !strings.Contains(out, "PROJ-1 moved to Done") {
		t.Errorf("expected transition confirmation, got: %s", out)
	}
}

func TestBrowseWithQuickActions_NoActionIsQuiet(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no API calls expected when no quick action is taken")
	}))
	defer srv.Close()
	defer mockShowTableQuickAction(tui.TableResult{Action: tui.ActionNone})()

	conn := api.JiraConnection{BaseURL: srv.URL, Email: "a@b.com", APIToken: testAPIToken}
	browseWithQuickActions(conn, nil, nil)
}

func TestRunMenu_CancelledImmediately(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no API calls expected when menu is cancelled immediately")
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	old := selectMenuOptionFn
	selectMenuOptionFn = func(title string, options []tui.MenuOption) (int, bool, error) {
		return 0, true, nil
	}
	defer func() { selectMenuOptionFn = old }()

	runMenu()
}
