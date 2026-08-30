package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
