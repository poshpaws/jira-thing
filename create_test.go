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

func TestRunCreate_FieldFlagsOverrideTemplate(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/myself"):
			json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-999"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issue"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			capturedFields, _ = body["fields"].(map[string]any)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-200"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"},"summary":"S","priority":{"name":"Low"}}`), 0o644)

	captureStdout(func() {
		runCreate([]string{
			"-t", tmplPath, "-y",
			"-type", "Bug", "-priority", "High", "-labels", "a, b",
			"-components", "Backend, API", "-assignee", "me",
		})
	})

	if it, ok := capturedFields["issuetype"].(map[string]any); !ok || it["name"] != "Bug" {
		t.Errorf("issuetype = %v, want Bug", capturedFields["issuetype"])
	}
	if p, ok := capturedFields["priority"].(map[string]any); !ok || p["name"] != "High" {
		t.Errorf("priority = %v, want High (should override template's Low)", capturedFields["priority"])
	}
	labels, _ := capturedFields["labels"].([]any)
	if len(labels) != 2 || labels[0] != "a" || labels[1] != "b" {
		t.Errorf("labels = %v", capturedFields["labels"])
	}
	components, _ := capturedFields["components"].([]any)
	if len(components) != 2 {
		t.Fatalf("components = %v, want 2 entries", capturedFields["components"])
	}
	if c0, ok := components[0].(map[string]any); !ok || c0["name"] != "Backend" {
		t.Errorf("components[0] = %v, want Backend", components[0])
	}
	assignee, ok := capturedFields["assignee"].(map[string]any)
	if !ok || assignee["accountId"] != "acc-999" {
		t.Errorf("assignee = %v, want resolved acc-999", capturedFields["assignee"])
	}
}

func TestRunCreate_EpicFlagLinksAfterCreate(t *testing.T) {
	var sawParentLink bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/field"):
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issue"):
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-201"})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/issue/PROJ-201"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			fields, _ := body["fields"].(map[string]any)
			if parent, ok := fields["parent"].(map[string]any); ok && parent["key"] == "EPIC-9" {
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
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"},"summary":"S","description":"D"}`), 0o644)

	captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y", "-epic", "EPIC-9"}) })
	if !sawParentLink {
		t.Error("expected -epic to link the new ticket to the given parent")
	}
}

func TestRunCreate_JSONOutputsResult(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-300"})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"},"summary":"S"}`), 0o644)

	out := captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y", "-json"}) })
	var got jsonResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if got.Key != "PROJ-300" || got.Status != "created" || got.URL != srv.URL+"/browse/PROJ-300" {
		t.Errorf("got %+v", got)
	}
}

func TestRunCreate_DryRunMakesNoCreateRequest(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request in dry-run mode: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"},"summary":"S","description":"D"}`), 0o644)

	out := captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y", "-dry-run"}) })
	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected dry-run notice, got: %s", out)
	}
	if !strings.Contains(out, `"summary": "S"`) {
		t.Errorf("expected resolved summary in printed payload, got: %s", out)
	}
	if !strings.Contains(out, "POST") || !strings.Contains(out, "/rest/api/3/issue") {
		t.Errorf("expected method and target URL in output, got: %s", out)
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

func TestRunCreate_BatchDryRunMakesNoCreateRequest(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request in batch dry-run mode: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	batchPath := filepath.Join(dir, "batch.json")
	os.WriteFile(batchPath, []byte(`[
		{"issuetype":{"name":"Task"},"summary":"First"},
		{"issuetype":{"name":"Task"},"summary":"Second"}
	]`), 0o644)

	out := captureStdout(func() { runCreate([]string{"-f", batchPath, "-dry-run"}) })
	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected dry-run notice, got: %s", out)
	}
	if !strings.Contains(out, "First") || !strings.Contains(out, "Second") {
		t.Errorf("expected both resolved summaries in output, got: %s", out)
	}
	if strings.Contains(out, "ticket(s) created") {
		t.Errorf("expected no creation summary in dry-run mode, got: %s", out)
	}
}

func TestRunCreate_BatchDryRunContinuesPastResolutionError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request in batch dry-run mode: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	batchPath := filepath.Join(dir, "batch.json")
	os.WriteFile(batchPath, []byte(`[
		{"issuetype":{"name":"Task"}},
		{"issuetype":{"name":"Task"},"summary":"Second"}
	]`), 0o644)

	out := captureStderr(func() {
		captureStdout(func() { runCreate([]string{"-f", batchPath, "-dry-run"}) })
	})
	if !strings.Contains(out, "ticket 1") {
		t.Errorf("expected ticket 1's missing-summary error reported, got: %s", out)
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

// --- runCreate: field shapes and flag-only create ---

// captureCreateFields starts a fake Jira that records the fields of the
// create request, answering any epic-link lookups the run may make.
func captureCreateFields(t *testing.T, captured *map[string]any) func() {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/issue") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*captured, _ = body["fields"].(map[string]any)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "CRSS-1"})
	}))
	restore := mockCreds(srv.URL)
	return func() { restore(); srv.Close() }
}

func TestRunCreate_BareStringProjectSentAsKeyObject(t *testing.T) {
	var fields map[string]any
	defer captureCreateFields(t, &fields)()

	tmplPath := filepath.Join(t.TempDir(), "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"project":"CRSS","issuetype":"Bug","summary":"S"}`), 0o644)

	captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y"}) })
	project, _ := fields["project"].(map[string]any)
	issueType, _ := fields["issuetype"].(map[string]any)
	if project["key"] != "CRSS" || issueType["name"] != "Bug" {
		t.Errorf("expected project/issuetype wrapped as objects, got %v", fields)
	}
}

func TestRunCreate_BatchBareStringProjectSentAsKeyObject(t *testing.T) {
	var fields map[string]any
	defer captureCreateFields(t, &fields)()

	batchPath := filepath.Join(t.TempDir(), "batch.json")
	os.WriteFile(batchPath, []byte(`[{"project":"CRSS","issuetype":{"name":"Bug"},"summary":"S"}]`), 0o644)

	captureStdout(func() { runCreate([]string{"-f", batchPath}) })
	if project, _ := fields["project"].(map[string]any); project["key"] != "CRSS" {
		t.Errorf("expected project {key: CRSS}, got %v", fields["project"])
	}
}

func TestRunCreate_FlagsOnlyWithoutTemplate(t *testing.T) {
	var fields map[string]any
	defer captureCreateFields(t, &fields)()
	defer withNoTemplateOnSearchPath(t)()

	captureStdout(func() {
		runCreate([]string{"-project", "CRSS", "-type", "Bug", "-summary", "Flag only", "-y"})
	})
	project, _ := fields["project"].(map[string]any)
	if project["key"] != "CRSS" || fields["summary"] != "Flag only" {
		t.Errorf("expected project and summary from flags, got %v", fields)
	}
}

func TestRunCreate_NoTemplateNoProjectStillPointsAtTemplate(t *testing.T) {
	defer withNoTemplateOnSearchPath(t)()

	out := captureStderr(func() {
		if !captureExit(func() { runCreate([]string{"-summary", "S", "-y"}) }) {
			t.Error("expected exit when there is neither a template nor -project")
		}
	})
	if !strings.Contains(out, "no template found") {
		t.Errorf("expected template guidance, got: %s", out)
	}
}

func TestRunCreate_DryRunShowsEpicLinkStep(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request in dry-run mode: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	tmplPath := filepath.Join(t.TempDir(), "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"project":"CRSS","summary":"S"}`), 0o644)

	out := captureStdout(func() { runCreate([]string{"-t", tmplPath, "-y", "-epic", "CRSS-12351", "-dry-run"}) })
	if !strings.Contains(out, `"key": "CRSS"`) {
		t.Errorf("expected nested project object in payload, got: %s", out)
	}
	if !strings.Contains(out, "CRSS-12351") {
		t.Errorf("expected the follow-up epic link step, got: %s", out)
	}
}
