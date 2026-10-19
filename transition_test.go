package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func transitionsHandler(t *testing.T, onPost func(body map[string]any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/transitions"):
			json.NewEncoder(w).Encode(map[string]any{
				"transitions": []map[string]any{
					{"id": "11", "name": "In Progress"},
					{"id": "21", "name": "Done"},
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/transitions"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if onPost != nil {
				onPost(body)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}
}

func TestRunTransition_NoStateListsAvailable(t *testing.T) {
	srv := httptest.NewTLSServer(transitionsHandler(t, nil))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runTransition([]string{"PROJ-1"}) })
	if !strings.Contains(out, "In Progress") || !strings.Contains(out, "Done") {
		t.Errorf("expected both state names listed, got: %s", out)
	}
}

func TestRunTransition_MovesToNamedStateCaseInsensitive(t *testing.T) {
	var postedID string
	srv := httptest.NewTLSServer(transitionsHandler(t, func(body map[string]any) {
		transition, _ := body["transition"].(map[string]any)
		postedID, _ = transition["id"].(string)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runTransition([]string{"PROJ-1", "done"}) })
	if postedID != "21" {
		t.Errorf("expected transition id 21 (Done), got %q", postedID)
	}
	if !strings.Contains(out, "PROJ-1") || !strings.Contains(out, "Done") {
		t.Errorf("expected success message naming ticket and state, got: %s", out)
	}
}

func TestRunTransition_UnknownStateExitsWithAvailableList(t *testing.T) {
	srv := httptest.NewTLSServer(transitionsHandler(t, nil))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStderr(func() {
		exited := captureExit(func() { runTransition([]string{"PROJ-1", "Bogus State"}) })
		if !exited {
			t.Error("expected osExit for unknown state")
		}
	})
	if !strings.Contains(out, "In Progress") || !strings.Contains(out, "Done") {
		t.Errorf("expected available states listed in error, got: %s", out)
	}
}

func TestRunTransition_ResolutionAndCommentIncludedInPayload(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewTLSServer(transitionsHandler(t, func(body map[string]any) {
		captured = body
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	captureStdout(func() {
		runTransition([]string{"PROJ-1", "Done", "-resolution", "Fixed", "-comment", "all set"})
	})
	fields, _ := captured["fields"].(map[string]any)
	resolution, _ := fields["resolution"].(map[string]any)
	if resolution["name"] != "Fixed" {
		t.Errorf("expected resolution Fixed, got %v", fields["resolution"])
	}
	update, _ := captured["update"].(map[string]any)
	if update == nil {
		t.Errorf("expected a comment update in payload, got: %v", captured)
	}
}

func TestRunTransition_DryRunMakesNoPostRequest(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/transitions") {
			json.NewEncoder(w).Encode(map[string]any{
				"transitions": []map[string]any{{"id": "21", "name": "Done"}},
			})
			return
		}
		t.Errorf("unexpected request in dry-run mode: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runTransition([]string{"PROJ-1", "Done", "-dry-run"}) })
	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected dry-run notice, got: %s", out)
	}
	if !strings.Contains(out, "POST") || !strings.Contains(out, "/transitions") {
		t.Errorf("expected method and target URL in output, got: %s", out)
	}
}
