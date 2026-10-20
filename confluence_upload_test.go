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
	"jira-thing/internal/tui"
)

// mockConfUploadConfig writes a temp config file with the Confluence fields
// conf upload requires, pointed at srv, and returns a cleanup func.
func mockConfUploadConfig(t *testing.T, srv *httptest.Server) func() {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "jira-thing.json")
	data := `{"confluence_space":"ENG","confluence_url":"` + srv.URL + `","confluence_base_page_id":"1"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	return func() { config.ConfigPath = old }
}

func writeTestMarkdown(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("# Hello\n\nBody text.\n"), 0o644); err != nil {
		t.Fatalf("writing markdown: %v", err)
	}
	return path
}

func TestRunConfUpload_PageIDUpdatesDirectlyNoBrowser(t *testing.T) {
	var putBody map[string]any
	var browserCalled bool
	origBrowse := browseSpaceFn
	defer func() { browseSpaceFn = origBrowse }()
	browseSpaceFn = func(root tui.PageEntry, fetch tui.PageFetcher) (tui.BrowseResult, error) {
		browserCalled = true
		return tui.BrowseResult{}, nil
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/content/42"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "42", "title": "Existing Title", "version": map[string]any{"number": 3},
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/content/42"):
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfUploadConfig(t, srv)()

	mdPath := writeTestMarkdown(t)
	out := captureStdout(func() { runConfUpload([]string{"-page-id", "42", mdPath}) })

	if browserCalled {
		t.Error("expected the parent-picker TUI not to be launched with -page-id")
	}
	if putBody["title"] != "Existing Title" {
		t.Errorf("expected existing title preserved, got: %v", putBody["title"])
	}
	version, _ := putBody["version"].(map[string]any)
	if version["number"] != float64(4) {
		t.Errorf("expected version bumped to 4, got: %v", version["number"])
	}
	if !strings.Contains(out, "42") {
		t.Errorf("expected page ID in output, got: %s", out)
	}
}

func TestRunConfUpload_PageIDWithTitleRenames(t *testing.T) {
	var putBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/content/42"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "42", "title": "Old Title", "version": map[string]any{"number": 1},
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/content/42"):
			json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfUploadConfig(t, srv)()

	mdPath := writeTestMarkdown(t)
	captureStdout(func() { runConfUpload([]string{"-page-id", "42", "-title", "New Title", mdPath}) })

	if putBody["title"] != "New Title" {
		t.Errorf("expected explicit -title to rename the page, got: %v", putBody["title"])
	}
}

func TestRunConfUpload_ParentIDCreatesWithoutBrowser(t *testing.T) {
	var browserCalled bool
	origBrowse := browseSpaceFn
	defer func() { browseSpaceFn = origBrowse }()
	browseSpaceFn = func(root tui.PageEntry, fetch tui.PageFetcher) (tui.BrowseResult, error) {
		browserCalled = true
		return tui.BrowseResult{}, nil
	}

	var createBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/content/99"):
			json.NewEncoder(w).Encode(map[string]any{
				"id": "99", "title": "Parent Page", "version": map[string]any{"number": 1},
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/content/99/child/page"):
			json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/content"):
			json.NewDecoder(r.Body).Decode(&createBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"id": "100", "title": "doc", "version": map[string]any{"number": 1}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfUploadConfig(t, srv)()

	mdPath := writeTestMarkdown(t)
	captureStdout(func() { runConfUpload([]string{"-parent-id", "99", mdPath}) })

	if browserCalled {
		t.Error("expected the parent-picker TUI not to be launched with -parent-id")
	}
	ancestors, _ := createBody["ancestors"].([]any)
	if len(ancestors) != 1 {
		t.Fatalf("expected one ancestor, got: %v", createBody["ancestors"])
	}
	ancestor, _ := ancestors[0].(map[string]any)
	if ancestor["id"] != "99" {
		t.Errorf("expected ancestor id 99, got: %v", ancestor["id"])
	}
}
