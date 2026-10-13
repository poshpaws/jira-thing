package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// --- runCreate: declarative / non-interactive ---

func TestRunCreate_UsesTemplateSummaryAndDescriptionWithoutPrompting(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		capturedFields, _ = body["fields"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-100"})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"},"summary":"From template","description":"Body text"}`), 0o644)

	// -y with no stdin available proves the template's own fields were used
	// and no prompt was attempted.
	out := captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y"}) })
	if !strings.Contains(out, "PROJ-100") {
		t.Errorf("expected PROJ-100 in output, got: %s", out)
	}
	if capturedFields["summary"] != "From template" {
		t.Errorf("summary = %v, want %q", capturedFields["summary"], "From template")
	}
	if _, ok := capturedFields["description"].(map[string]any); !ok {
		t.Errorf("description = %v, want an ADF doc", capturedFields["description"])
	}
}

func TestRunCreate_NonInteractiveMissingSummaryExits(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"}}`), 0o644)

	exited := captureExit(func() {
		captureStderr(func() { runCreate([]string{"-t", tmplPath, "-non-interactive"}) })
	})
	if !exited {
		t.Error("expected exit on missing summary in non-interactive mode")
	}
}

func TestRunCreate_DescriptionFileOverridesTemplate(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		capturedFields, _ = body["fields"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-101"})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"},"summary":"S","description":"ignored"}`), 0o644)
	descPath := filepath.Join(dir, "desc.md")
	os.WriteFile(descPath, []byte("# From file"), 0o644)

	captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y", "-description-file", descPath}) })
	desc, ok := capturedFields["description"].(map[string]any)
	if !ok {
		t.Fatalf("description = %v, want an ADF doc", capturedFields["description"])
	}
	if b, _ := json.Marshal(desc); !strings.Contains(string(b), "From file") {
		t.Errorf("description should come from file, got: %v", desc)
	}
}

func TestRunCreate_ParentLinksAfterCreate(t *testing.T) {
	var sawParentLink bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/field"):
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issue"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-102"})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/issue/PROJ-102"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			fields, _ := body["fields"].(map[string]any)
			if parent, ok := fields["parent"].(map[string]any); ok && parent["key"] == "EPIC-1" {
				sawParentLink = true
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"},"summary":"S","description":"D","parent":"EPIC-1"}`), 0o644)

	captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y"}) })
	if !sawParentLink {
		t.Error("expected a follow-up request linking the new ticket to its parent")
	}
}

// --- runCreate: batch ---

func TestRunCreate_BatchCreatesAllAndReportsFailures(t *testing.T) {
	created := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		created++
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-" + strconv.Itoa(created)})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	batchPath := filepath.Join(dir, "batch.json")
	os.WriteFile(batchPath, []byte(`[
		{"issuetype":{"name":"Task"},"summary":"First"},
		{"issuetype":{"name":"Task"}},
		{"issuetype":{"name":"Task"},"summary":"Third"}
	]`), 0o644)

	out := captureStdout(func() { runCreate([]string{"-f", batchPath}) })
	if !strings.Contains(out, "2 of 3 ticket(s) created") {
		t.Errorf("expected partial-success summary, got: %s", out)
	}
	if created != 2 {
		t.Errorf("expected 2 create requests, got %d", created)
	}
}

func TestRunCreate_BatchEmptyArrayExits(t *testing.T) {
	dir := t.TempDir()
	batchPath := filepath.Join(dir, "batch.json")
	os.WriteFile(batchPath, []byte(`[]`), 0o644)

	exited := captureExit(func() {
		captureStderr(func() { runCreate([]string{"-f", batchPath}) })
	})
	if !exited {
		t.Error("expected exit on empty batch file")
	}
}
