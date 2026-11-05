package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dedupeJira fakes search and create: existing maps a summary to the key of
// an open ticket, and creates records each created summary.
type dedupeJira struct {
	existing   map[string]string
	searchFail bool
	searches   int
	creates    []string
}

func (j *dedupeJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	if strings.HasSuffix(r.URL.Path, "/search/jql") {
		j.serveSearch(w)
		return
	}
	fields, _ := body["fields"].(map[string]any)
	summary, _ := fields["summary"].(string)
	j.creates = append(j.creates, summary)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"key": "NEW-1"})
}

// serveSearch answers with every existing ticket; the code under test does
// the exact-summary filtering, just as with Jira's fuzzy match.
func (j *dedupeJira) serveSearch(w http.ResponseWriter) {
	j.searches++
	if j.searchFail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	issues := []map[string]any{}
	for summary, key := range j.existing {
		issues = append(issues, map[string]any{"key": key, "fields": map[string]any{"summary": summary}})
	}
	json.NewEncoder(w).Encode(map[string]any{"issues": issues})
}

// startDedupeJira serves j and points credentials at it.
func startDedupeJira(j *dedupeJira) func() {
	srv := httptest.NewTLSServer(j)
	restore := mockCreds(srv.URL)
	return func() { restore(); srv.Close() }
}

// writeFile writes content to name in a fresh temp directory, returning its path.
func writeFile(t *testing.T, name, content string) string {
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const dedupeTemplate = `{"project":"PROJ","issuetype":"Task","summary":"Fix login"}`

func TestRunCreate_DedupeSkipsWhenOpenDuplicateExists(t *testing.T) {
	j := &dedupeJira{existing: map[string]string{"Fix login": "PROJ-7"}}
	defer startDedupeJira(j)()

	out := captureStdout(func() {
		runCreate([]string{"-t", writeFile(t, "t.json", dedupeTemplate), "-y", "-dedupe-by-summary"})
	})
	if len(j.creates) != 0 {
		t.Errorf("expected no create, got %v", j.creates)
	}
	if !strings.Contains(out, "PROJ-7") {
		t.Errorf("expected notice naming PROJ-7, got: %s", out)
	}
}

func TestRunCreate_DedupeCreatesWhenNoDuplicate(t *testing.T) {
	j := &dedupeJira{}
	defer startDedupeJira(j)()

	captureStdout(func() {
		runCreate([]string{"-t", writeFile(t, "t.json", dedupeTemplate), "-y", "-dedupe-by-summary"})
	})
	if len(j.creates) != 1 {
		t.Errorf("expected one create, got %v", j.creates)
	}
}

func TestRunCreate_DedupeJSONReportsExisting(t *testing.T) {
	j := &dedupeJira{existing: map[string]string{"Fix login": "PROJ-7"}}
	defer startDedupeJira(j)()

	out := captureStdout(func() {
		runCreate([]string{"-t", writeFile(t, "t.json", dedupeTemplate), "-y", "-dedupe-by-summary", "-json"})
	})
	var got jsonResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if got.Key != "PROJ-7" || got.Status != "exists" {
		t.Errorf("got %+v", got)
	}
}

func TestRunCreate_DedupeSearchFailureCreatesNothing(t *testing.T) {
	j := &dedupeJira{searchFail: true}
	defer startDedupeJira(j)()

	tmpl := writeFile(t, "t.json", dedupeTemplate)
	captureStderr(func() {
		if !captureExit(func() { runCreate([]string{"-t", tmpl, "-y", "-dedupe-by-summary"}) }) {
			t.Error("expected exit when the duplicate check fails")
		}
	})
	if len(j.creates) != 0 {
		t.Errorf("expected no create, got %v", j.creates)
	}
}

func TestRunCreate_DedupeWithoutProjectExits(t *testing.T) {
	j := &dedupeJira{}
	defer startDedupeJira(j)()

	tmpl := writeFile(t, "t.json", `{"summary":"S"}`)
	out := captureStderr(func() {
		if !captureExit(func() { runCreate([]string{"-t", tmpl, "-y", "-dedupe-by-summary"}) }) {
			t.Error("expected exit when no project is known")
		}
	})
	if !strings.Contains(out, "project") || j.searches != 0 || len(j.creates) != 0 {
		t.Errorf("searches=%d creates=%v stderr=%s", j.searches, j.creates, out)
	}
}

func TestRunCreate_NoDedupeFlagDoesNotSearch(t *testing.T) {
	j := &dedupeJira{existing: map[string]string{"Fix login": "PROJ-7"}}
	defer startDedupeJira(j)()

	captureStdout(func() { runCreate([]string{"-t", writeFile(t, "t.json", dedupeTemplate), "-y"}) })
	if j.searches != 0 || len(j.creates) != 1 {
		t.Errorf("searches=%d creates=%v", j.searches, j.creates)
	}
}

func TestRunCreate_DedupeDryRunReportsDuplicate(t *testing.T) {
	j := &dedupeJira{existing: map[string]string{"Fix login": "PROJ-7"}}
	defer startDedupeJira(j)()

	out := captureStdout(func() {
		runCreate([]string{"-t", writeFile(t, "t.json", dedupeTemplate), "-y", "-dedupe-by-summary", "-dry-run"})
	})
	if !strings.Contains(out, "PROJ-7") || len(j.creates) != 0 {
		t.Errorf("creates=%v output: %s", j.creates, out)
	}
}

const dedupeBatch = `[
	{"project":"PROJ","issuetype":"Task","summary":"Fix login"},
	{"project":"PROJ","issuetype":"Task","summary":"New thing","description":"Body"},
	{"project":"PROJ","issuetype":"Task","summary":"new THING"},
	{"project":"OTHER","issuetype":"Task","summary":"New thing"}
]`

func TestRunCreate_BatchDedupeSkipsExistingAndRepeats(t *testing.T) {
	j := &dedupeJira{existing: map[string]string{"Fix login": "PROJ-7"}}
	defer startDedupeJira(j)()

	out := captureStdout(func() {
		runCreate([]string{"-f", writeFile(t, "b.json", dedupeBatch), "-dedupe-by-summary"})
	})
	if len(j.creates) != 2 {
		t.Errorf("expected New thing in PROJ and OTHER only, got %v", j.creates)
	}
	if !strings.Contains(out, "2 of 4") || !strings.Contains(out, "2 skipped") {
		t.Errorf("expected created and skipped counts, got: %s", out)
	}
}

func TestRunCreate_BatchDedupeSearchFailureSkipsThatEntry(t *testing.T) {
	j := &dedupeJira{searchFail: true}
	defer startDedupeJira(j)()

	batch := writeFile(t, "b.json", dedupeBatch)
	captureStderr(func() {
		captureStdout(func() { runCreate([]string{"-f", batch, "-dedupe-by-summary"}) })
	})
	if len(j.creates) != 0 {
		t.Errorf("expected no creates, got %v", j.creates)
	}
}

func TestRunCreate_BatchDedupeDryRunFlagsRepeats(t *testing.T) {
	j := &dedupeJira{}
	defer startDedupeJira(j)()

	out := captureStdout(func() {
		runCreate([]string{"-f", writeFile(t, "b.json", dedupeBatch), "-dedupe-by-summary", "-dry-run"})
	})
	if len(j.creates) != 0 {
		t.Errorf("dry run must not create, got %v", j.creates)
	}
	if !strings.Contains(out, "ticket 2 in this batch") {
		t.Errorf("expected entry 3 flagged as a repeat of entry 2, got: %s", out)
	}
}
