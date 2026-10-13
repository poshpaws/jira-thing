package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunEdit_UpdatesOnlyGivenFields(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			capturedFields, _ = body["fields"].(map[string]any)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() {
		runEdit([]string{"-summary", "New summary", "-priority", "High", "-labels", "a, b", "PROJ-7"})
	})
	if !strings.Contains(out, "PROJ-7") {
		t.Errorf("expected PROJ-7 in output, got: %s", out)
	}
	if capturedFields["summary"] != "New summary" {
		t.Errorf("summary = %v", capturedFields["summary"])
	}
	if p, ok := capturedFields["priority"].(map[string]any); !ok || p["name"] != "High" {
		t.Errorf("priority = %v", capturedFields["priority"])
	}
	labels, ok := capturedFields["labels"].([]any)
	if !ok || len(labels) != 2 || labels[0] != "a" || labels[1] != "b" {
		t.Errorf("labels = %v", capturedFields["labels"])
	}
	if _, ok := capturedFields["description"]; ok {
		t.Errorf("description should be untouched, got: %v", capturedFields["description"])
	}
}

// TestRunEdit_KeyFirstOrdering guards against a flag.Parse footgun: the
// stdlib flag package stops parsing at the first non-flag argument, so
// "edit PROJ-7 -priority High" (key before flags — the form documented in
// SKILL.md/README.md) would silently drop every flag after the key without
// the reorder step in runEdit.
func TestRunEdit_KeyFirstOrdering(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			capturedFields, _ = body["fields"].(map[string]any)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	captureStdout(func() { runEdit([]string{"PROJ-7", "-priority", "High"}) })
	p, ok := capturedFields["priority"].(map[string]any)
	if !ok || p["name"] != "High" {
		t.Errorf("priority = %v, want High (flag after key was likely dropped)", capturedFields["priority"])
	}
}

func TestRunEdit_AssigneeMeResolvesCurrentUser(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/myself"):
			json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-123"})
		case r.Method == http.MethodPut:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			capturedFields, _ = body["fields"].(map[string]any)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	captureStdout(func() { runEdit([]string{"-assignee", "me", "PROJ-8"}) })
	assignee, ok := capturedFields["assignee"].(map[string]any)
	if !ok || assignee["accountId"] != "acc-123" {
		t.Errorf("assignee = %v", capturedFields["assignee"])
	}
}

func TestRunEdit_ParentLinksWithoutFieldUpdate(t *testing.T) {
	var putCount int
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/field"):
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPut:
			putCount++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	captureStdout(func() { runEdit([]string{"-parent", "EPIC-5", "PROJ-9"}) })
	if putCount != 1 {
		t.Errorf("expected exactly one PUT (the parent link), got %d", putCount)
	}
}

func TestRunEdit_NothingToEditExits(t *testing.T) {
	defer mockCreds("https://example.invalid")()
	exited := captureExit(func() {
		captureStderr(func() { runEdit([]string{"PROJ-10"}) })
	})
	if !exited {
		t.Error("expected exit when no edit flags are given")
	}
}
