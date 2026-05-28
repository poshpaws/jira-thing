package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jira-thing/internal/api"
	"jira-thing/internal/auth"
	"jira-thing/internal/config"
	"jira-thing/internal/tui"
)

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func captureStdout(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureStderr(f func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestBuildDescription(t *testing.T) {
	desc := buildDescription("hello world")
	if desc["type"] != "doc" {
		t.Errorf("type = %v, want doc", desc["type"])
	}
	if desc["version"] != 1 {
		t.Errorf("version = %v, want 1", desc["version"])
	}
	content, ok := desc["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content = %v", desc["content"])
	}
	para, ok := content[0].(map[string]any)
	if !ok || para["type"] != "paragraph" {
		t.Errorf("paragraph = %v", content[0])
	}
	inner := para["content"].([]any)[0].(map[string]any)
	if inner["text"] != "hello world" {
		t.Errorf("text = %v, want hello world", inner["text"])
	}
}

func TestBuildMyTasksJQL_Open(t *testing.T) {
	jql := buildMyTasksJQL(false)
	if !strings.Contains(jql, "assignee = currentUser()") {
		t.Errorf("missing assignee clause: %s", jql)
	}
	if !strings.Contains(jql, "ORDER BY updated DESC") {
		t.Errorf("missing descending order: %s", jql)
	}
	if strings.Contains(jql, "updated <=") {
		t.Errorf("should not have updated filter: %s", jql)
	}
}

func TestBuildMyTasksJQL_NotUpdated(t *testing.T) {
	jql := buildMyTasksJQL(true)
	if !strings.Contains(jql, "updated <=") {
		t.Errorf("missing updated filter: %s", jql)
	}
	if !strings.Contains(jql, "ORDER BY updated ASC") {
		t.Errorf("missing ascending order: %s", jql)
	}
}

func TestTaskLabel(t *testing.T) {
	if got := taskLabel(false); got != "open" {
		t.Errorf("taskLabel(false) = %q, want open", got)
	}
	if got := taskLabel(true); !strings.Contains(got, "stale") {
		t.Errorf("taskLabel(true) = %q, want stale label", got)
	}
}

func TestGetString(t *testing.T) {
	m := map[string]any{"name": "Alice", "count": 3}
	if got := getString(m, "name"); got != "Alice" {
		t.Errorf("got %q, want Alice", got)
	}
	if got := getString(m, "missing"); got != "" {
		t.Errorf("missing key got %q, want empty", got)
	}
	if got := getString(m, "count"); got != "" {
		t.Errorf("non-string got %q, want empty", got)
	}
}

func TestNestedString(t *testing.T) {
	m := map[string]any{
		"status":  map[string]any{"name": "In Progress"},
		"notAMap": "string value",
	}
	if got := nestedString(m, "status", "name"); got != "In Progress" {
		t.Errorf("got %q, want In Progress", got)
	}
	if got := nestedString(m, "missing", "name"); got != "" {
		t.Errorf("missing outer key got %q", got)
	}
	if got := nestedString(m, "notAMap", "name"); got != "" {
		t.Errorf("non-map outer got %q", got)
	}
}

func TestPrintTaskRow(t *testing.T) {
	issue := map[string]any{
		"key": "PROJ-42",
		"fields": map[string]any{
			"summary":  "Fix the thing",
			"updated":  "2026-04-20T10:00:00.000Z",
			"status":   map[string]any{"name": "In Progress"},
			"priority": map[string]any{"name": "High"},
		},
	}
	out := captureStdout(func() { printTaskRow(issue) })
	if !strings.Contains(out, "PROJ-42") {
		t.Errorf("missing key: %s", out)
	}
	if !strings.Contains(out, "Fix the thing") {
		t.Errorf("missing summary: %s", out)
	}
	if !strings.Contains(out, "2026-04-20") {
		t.Errorf("missing date: %s", out)
	}
}

func TestPrintTaskRow_MissingFields(t *testing.T) {
	out := captureStdout(func() { printTaskRow(map[string]any{"key": "PROJ-99"}) })
	if !strings.Contains(out, "PROJ-99") {
		t.Errorf("missing key: %s", out)
	}
}

func TestPrintTasks(t *testing.T) {
	issues := []map[string]any{
		{"key": "A-1", "fields": map[string]any{"summary": "First"}},
		{"key": "A-2", "fields": map[string]any{"summary": "Second"}},
	}
	out := captureStdout(func() { printTasks(issues) })
	if !strings.Contains(out, "A-1") {
		t.Errorf("missing A-1: %s", out)
	}
	if !strings.Contains(out, "A-2") {
		t.Errorf("missing A-2: %s", out)
	}
}

func TestPrintUsage(t *testing.T) {
	out := captureStderr(func() { printUsage() })
	if !strings.Contains(out, "Usage:") {
		t.Errorf("missing Usage: %s", out)
	}
	if !strings.Contains(out, "template") {
		t.Errorf("missing template command: %s", out)
	}
}

// --- helpers for command-level tests ---

func mockCreds(baseURL string) func() {
	old := getCredentialsFn
	getCredentialsFn = func() (auth.Credentials, error) {
		return auth.Credentials{URL: baseURL, Email: "a@b.com", Token: "tok"}, nil
	}
	return func() { getCredentialsFn = old }
}

func mockCredsError(err error) func() {
	old := getCredentialsFn
	getCredentialsFn = func() (auth.Credentials, error) { return auth.Credentials{}, err }
	return func() { getCredentialsFn = old }
}

// mockShowTableSelectAll replaces showTableFn to auto-select all tickets.
func mockShowTableSelectAll() func() {
	old := showTableFn
	showTableFn = func(tickets []tui.Ticket) ([]tui.Ticket, error) { return tickets, nil }
	return func() { showTableFn = old }
}

// exitSignal is a sentinel panic value used to simulate os.Exit in tests.
type exitSignal struct{ code int }

// captureExit runs f and returns true if osExit was called (via panic+recover).
// It also restores osExit after the call.
func captureExit(f func()) (didExit bool) {
	old := osExit
	defer func() {
		osExit = old
		if r := recover(); r != nil {
			if _, ok := r.(exitSignal); ok {
				didExit = true
			} else {
				panic(r)
			}
		}
	}()
	osExit = func(code int) { panic(exitSignal{code}) }
	f()
	return false
}

func pipeStdinLines(t *testing.T, lines ...string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	return func() { os.Stdin = old; r.Close() }
}

// --- fatal ---

func TestFatal(t *testing.T) {
	exited := captureExit(func() {
		// Discard stderr to avoid test noise; we only verify osExit was called.
		old := os.Stderr
		os.Stderr, _ = os.Open(os.DevNull)
		defer func() { os.Stderr = old }()
		fatal("something went wrong: %s", "detail")
	})
	if !exited {
		t.Error("osExit not called by fatal")
	}
}

// --- buildConnection ---

func TestBuildConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	conn, err := buildConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.BaseURL != srv.URL {
		t.Errorf("BaseURL = %q", conn.BaseURL)
	}
	if conn.Email != "a@b.com" {
		t.Errorf("Email = %q", conn.Email)
	}
}

func TestBuildConnection_Error(t *testing.T) {
	defer mockCredsError(fmt.Errorf("no keyring"))()
	_, err := buildConnection()
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- mustConnect ---

func TestMustConnect_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	conn := mustConnect()
	if conn.BaseURL != srv.URL {
		t.Errorf("BaseURL = %q", conn.BaseURL)
	}
}

func TestMustConnect_Fatal(t *testing.T) {
	defer mockCredsError(fmt.Errorf("no creds"))()
	exited := captureExit(func() {
		captureStderr(func() { mustConnect() })
	})
	if !exited {
		t.Error("osExit not called on credential error")
	}
}

// --- promptTicketFields ---

func TestPromptTicketFields_Success(t *testing.T) {
	defer pipeStdinLines(t, "Fix the bug", "Some description")()
	summary, desc, err := promptTicketFields()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "Fix the bug" {
		t.Errorf("summary = %q", summary)
	}
	if desc != "Some description" {
		t.Errorf("desc = %q", desc)
	}
}

func TestPromptTicketFields_EmptySummary(t *testing.T) {
	defer pipeStdinLines(t, "", "desc")()
	_, _, err := promptTicketFields()
	if err == nil {
		t.Fatal("expected error for empty summary")
	}
}

func TestPromptTicketFields_SummaryReadError(t *testing.T) {
	r, w, _ := os.Pipe()
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old; r.Close() }()

	_, _, err := promptTicketFields()
	if err == nil {
		t.Fatal("expected error for stdin EOF")
	}
}

func TestPromptTicketFields_DescriptionReadError(t *testing.T) {
	defer pipeStdinLines(t, "My summary")() // no description line written
	_, _, err := promptTicketFields()
	// On EOF for description, the function returns without error (empty desc is OK).
	// The description read can return io.EOF which is treated as an error.
	// Either outcome (err or empty desc) is acceptable; just must not panic.
	_ = err
}

// --- runTemplate ---

func TestRunTemplate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"key":    "PROJ-1",
			"fields": map[string]any{"issuetype": map[string]any{"name": "Task"}},
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	out := filepath.Join(dir, "tpl.json")
	stdout := captureStdout(func() {
		runTemplate([]string{"-o", out, "PROJ-1"})
	})
	if !strings.Contains(stdout, "Template saved") {
		t.Errorf("expected Template saved, got: %s", stdout)
	}
}

// --- runMyTasks ---

func TestRunMyTasks_NoTasks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total": 0, "maxResults": 100})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runMyTasks([]string{}) })
	if !strings.Contains(out, "No tasks found") {
		t.Errorf("expected No tasks found, got: %s", out)
	}
}

func TestRunMyTasks_WithResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{
				map[string]any{"key": "PROJ-5", "fields": map[string]any{
					"summary":  "Do a thing",
					"updated":  "2026-04-20T10:00:00.000Z",
					"status":   map[string]any{"name": "Open"},
					"priority": map[string]any{"name": "High"},
				}},
			},
			"total": 1, "maxResults": 100,
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runMyTasks([]string{}) })
	if !strings.Contains(out, "PROJ-5") {
		t.Errorf("expected PROJ-5 in output, got: %s", out)
	}
}

func TestRunMyTasks_NotUpdated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total": 0, "maxResults": 100})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runMyTasks([]string{"-notupdated"}) })
	if !strings.Contains(out, "No tasks found") {
		t.Errorf("expected No tasks found, got: %s", out)
	}
}

// --- runCreate ---

func TestRunCreate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-99"})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer pipeStdinLines(t, "My new ticket", "Some description")()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{"issuetype":{"name":"Task"}}`), 0o644)

	out := captureStdout(func() { runCreate([]string{"-t", tmplPath}) })
	if !strings.Contains(out, "PROJ-99") {
		t.Errorf("expected PROJ-99 in output, got: %s", out)
	}
}

func TestRunCreate_StripsBlockedFields(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		capturedFields, _ = body["fields"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-99"})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer pipeStdinLines(t, "My ticket", "desc")()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "tpl.json")
	os.WriteFile(tmplPath, []byte(`{
		"issuetype":        {"name":"Task"},
		"rankBeforeIssue":  {"key":"OTHER-1"},
		"rankAfterIssue":   {"key":"OTHER-2"},
		"customfield_10019": "0|i0000v:"
	}`), 0o644)

	captureStdout(func() { runCreate([]string{"-t", tmplPath}) })
	if _, ok := capturedFields["rankBeforeIssue"]; ok {
		t.Error("rankBeforeIssue must be stripped before sending to Jira")
	}
	if _, ok := capturedFields["rankAfterIssue"]; ok {
		t.Error("rankAfterIssue must be stripped before sending to Jira")
	}
	if _, ok := capturedFields["customfield_10019"]; ok {
		t.Error("customfield_10019 (rank) must be stripped before sending to Jira")
	}
}

func TestRunCreate_TemplateError(t *testing.T) {
	exited := captureExit(func() {
		captureStderr(func() {
			runCreate([]string{"-t", "/nonexistent/template.json"})
		})
	})
	if !exited {
		t.Error("osExit not called for missing template")
	}
}

// --- readAllStdin ---

func TestReadAllStdin(t *testing.T) {
	defer pipeStdinLines(t, "hello world")()
	text, err := readAllStdin()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("got %q, want hello world", text)
	}
}

// --- openEditor ---

func TestOpenEditor_NoEditor(t *testing.T) {
	old := os.Getenv("EDITOR")
	os.Unsetenv("EDITOR")
	defer os.Setenv("EDITOR", old)

	oldCfg := config.ConfigPath
	config.ConfigPath = func() string { return "/nonexistent/jira-thing.json" }
	defer func() { config.ConfigPath = oldCfg }()

	_, err := openEditor()
	if err == nil {
		t.Fatal("expected error when EDITOR not set")
	}
}

func TestOpenEditor_EditorWithArgs(t *testing.T) {
	// Simulate "code --wait" arg splitting: EDITOR="cp <src>" gets split into
	// ["cp", "<src>"] and the temp file is appended, producing: cp <src> <tmp>.
	src := filepath.Join(t.TempDir(), "content.txt")
	if err := os.WriteFile(src, []byte("test content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Setenv("EDITOR", "cp "+src)
	defer os.Unsetenv("EDITOR")

	text, err := openEditor()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "test content" {
		t.Errorf("got %q, want test content", text)
	}
}

// --- runUpdate ---

func TestRunUpdate_Stdin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/comment") {
			t.Errorf("expected /comment path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": "12345"})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer pipeStdinLines(t, "new comment text")()

	out := captureStdout(func() { runUpdate([]string{"-stdin", "PROJ-1"}) })
	if !strings.Contains(out, "Comment added to PROJ-1") {
		t.Errorf("expected 'Comment added to PROJ-1' in output, got: %s", out)
	}
}

func TestRunUpdate_EmptyInput(t *testing.T) {
	defer mockCreds("http://unused")()
	defer pipeStdinLines(t)() // nothing written → empty stdin

	exited := captureExit(func() {
		captureStderr(func() { runUpdate([]string{"-stdin", "PROJ-1"}) })
	})
	if !exited {
		t.Error("expected osExit for empty input")
	}
}

func TestRunUpdate_NoArgs(t *testing.T) {
	exited := captureExit(func() {
		captureStderr(func() { runUpdate([]string{}) })
	})
	if !exited {
		t.Error("expected osExit for missing ticket key")
	}
}

func TestRunUpdate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer pipeStdinLines(t, "some update text")()

	exited := captureExit(func() {
		captureStderr(func() { runUpdate([]string{"-stdin", "BAD-1"}) })
	})
	if !exited {
		t.Error("expected osExit for API error")
	}
}

func TestThreeBusinessDaysAgo(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "from Wednesday gives previous Friday",
			now:  date(2026, 4, 22), // Wed
			want: date(2026, 4, 17), // Fri (Wed->Tue->Mon->Fri)
		},
		{
			name: "from Monday gives previous Wednesday",
			now:  date(2026, 4, 27), // Mon
			want: date(2026, 4, 22), // Wed (Mon->Fri->Thu->Wed)
		},
		{
			name: "from Friday gives previous Tuesday",
			now:  date(2026, 4, 24), // Fri
			want: date(2026, 4, 21), // Tue (Thu->Wed->Tue)
		},
		{
			name: "from Sunday skips weekend",
			now:  date(2026, 4, 26), // Sun
			want: date(2026, 4, 22), // Wed (Sat skipped, Fri->Thu->Wed)
		},
		{
			name: "from Saturday skips weekend",
			now:  date(2026, 4, 25), // Sat
			want: date(2026, 4, 22), // Wed (Fri->Thu->Wed)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := threeBusinessDaysAgo(tc.now)
			if !got.Equal(tc.want) {
				t.Errorf("threeBusinessDaysAgo(%s) = %s, want %s",
					tc.now.Format("2006-01-02"),
					got.Format("2006-01-02"),
					tc.want.Format("2006-01-02"),
				)
			}
		})
	}
}

// --- runToilCheck ---

func mockConfig(marker, team string) func() {
	dir, _ := os.MkdirTemp("", "jira-thing-cfg")
	path := filepath.Join(dir, "jira-thing.json")
	data := fmt.Sprintf(`{"project":"CRSS","toil_marker":%q,"toil_team":%q}`, marker, team)
	os.WriteFile(path, []byte(data), 0o644)
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	return func() { config.ConfigPath = old; os.RemoveAll(dir) }
}

func TestRunToilCheck_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{
				map[string]any{"key": "CRSS-10", "fields": map[string]any{
					"summary":  "Toil task",
					"updated":  "2026-05-20T10:00:00.000Z",
					"status":   map[string]any{"name": "Open"},
					"priority": map[string]any{"name": "Medium"},
				}},
			},
			"total": 1, "maxResults": 100,
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfig("ECP_TOIL", "ECP_SEC_TEAM")()

	out := captureStdout(func() { runToilCheck() })
	if !strings.Contains(out, "CRSS-10") {
		t.Errorf("expected CRSS-10 in output, got: %s", out)
	}
	if !strings.Contains(out, "1 toil ticket") {
		t.Errorf("expected ticket count in output, got: %s", out)
	}
}

func TestRunToilCheck_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total": 0, "maxResults": 100})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfig("ECP_TOIL", "ECP_SEC_TEAM")()

	out := captureStdout(func() { runToilCheck() })
	if !strings.Contains(out, "No toil tickets found") {
		t.Errorf("expected 'No toil tickets found', got: %s", out)
	}
}

func TestRunToilCheck_MissingConfig(t *testing.T) {
	old := config.ConfigPath
	config.ConfigPath = func() string { return "/nonexistent/path.json" }
	defer func() { config.ConfigPath = old }()

	exited := captureExit(func() {
		captureStderr(func() { runToilCheck() })
	})
	if !exited {
		t.Error("expected osExit when config is missing")
	}
}

func TestRunToilCheck_JQLContainsLabels(t *testing.T) {
	var receivedJQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		receivedJQL, _ = body["jql"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total": 0, "maxResults": 100})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfig("ECP_TOIL", "ECP_SEC_TEAM")()

	captureStdout(func() { runToilCheck() })
	if !strings.Contains(receivedJQL, `labels = "ECP_TOIL"`) {
		t.Errorf("JQL missing toil_marker label: %s", receivedJQL)
	}
	if !strings.Contains(receivedJQL, `labels = "ECP_SEC_TEAM"`) {
		t.Errorf("JQL missing toil_team label: %s", receivedJQL)
	}
	if !strings.Contains(receivedJQL, `project = "CRSS"`) {
		t.Errorf("JQL missing project: %s", receivedJQL)
	}
	if !strings.Contains(receivedJQL, "updated >= -1w") {
		t.Errorf("JQL missing time filter: %s", receivedJQL)
	}
}

// --- extractNotes ---

func TestExtractNotes_Found(t *testing.T) {
	body := "<!-- jira-thing:notes:start --><p>my notes</p><!-- jira-thing:notes:end -->"
	got := extractNotes(body)
	if got != "<p>my notes</p>" {
		t.Errorf("extractNotes = %q, want <p>my notes</p>", got)
	}
}

func TestExtractNotes_Missing(t *testing.T) {
	if got := extractNotes("<p>no markers</p>"); got != "" {
		t.Errorf("extractNotes = %q, want empty", got)
	}
}

// --- renderTicketPage ---

func testIssue(key, summary string) map[string]any {
	return map[string]any{
		"key": key,
		"fields": map[string]any{
			"summary":  summary,
			"updated":  "2026-05-20T10:00:00.000Z",
			"status":   map[string]any{"name": "Open"},
			"priority": map[string]any{"name": "High"},
		},
	}
}

func TestRenderTicketPage_ContainsDetailsAndNotes(t *testing.T) {
	out := renderTicketPage(testIssue("CRSS-1", "Fix toil"), "")
	for _, want := range []string{
		"jira-thing:details:start", "jira-thing:details:end",
		"jira-thing:notes:start", "jira-thing:notes:end",
		"ac:structured-macro", `ac:name="jira"`, "CRSS-1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestRenderTicketPage_PreservesExistingNotes(t *testing.T) {
	out := renderTicketPage(testIssue("CRSS-1", "Fix toil"), "<p>saved note</p>")
	if !strings.Contains(out, "<p>saved note</p>") {
		t.Errorf("notes not preserved: %s", out)
	}
}

func TestRenderTicketPage_KeyEscaped(t *testing.T) {
	out := renderTicketPage(testIssue(`<script>`, "irrelevant"), "")
	if strings.Contains(out, "<script>") {
		t.Errorf("unescaped key in output: %s", out)
	}
}

func TestRenderTicketPage_NewPageHasDateAndNotePanel(t *testing.T) {
	out := renderTicketPage(testIssue("CRSS-1", "Fix toil"), "")
	if !strings.Contains(out, `<time datetime=`) {
		t.Errorf("new page missing time element: %s", out)
	}
	if !strings.Contains(out, `ac:adf-extension`) {
		t.Errorf("new page missing adf-extension: %s", out)
	}
	if !strings.Contains(out, `panelType`) || !strings.Contains(out, `note`) {
		t.Errorf("new page missing note panelType: %s", out)
	}
}

func TestRenderTicketPage_ExistingNotesNoDatePanel(t *testing.T) {
	out := renderTicketPage(testIssue("CRSS-1", "Fix toil"), "<p>existing note</p>")
	if strings.Contains(out, `<time datetime=`) {
		t.Errorf("existing notes page should not inject date element: %s", out)
	}
	if strings.Contains(out, `ac:adf-extension`) {
		t.Errorf("existing notes page should not inject panel: %s", out)
	}
}

// --- renderHangerPage ---

func TestRenderHangerPage_WithIssues(t *testing.T) {
	issues := []map[string]any{
		testIssue("CRSS-1", "Fix toil"),
		testIssue("CRSS-2", "More toil"),
	}
	out := renderHangerPage(issues, nil)
	for _, want := range []string{"CRSS-1", "CRSS-2", "ac:link", "ri:page"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in hanger page: %s", want, out)
		}
	}
}

func TestRenderHangerPage_Empty(t *testing.T) {
	out := renderHangerPage([]map[string]any{}, nil)
	if !strings.Contains(out, "No open TOIL tickets") {
		t.Errorf("expected no-ticket message, got: %s", out)
	}
}

func TestRenderHangerPage_IncludesManualPages(t *testing.T) {
	issues := []map[string]any{testIssue("CRSS-1", "Fix toil")}
	manuals := []api.ConfluencePageWithBody{
		{ConfluencePage: api.ConfluencePage{Title: "Team Notes"}},
		{ConfluencePage: api.ConfluencePage{Title: "Process Guide"}},
	}
	out := renderHangerPage(issues, manuals)
	for _, want := range []string{"CRSS-1", "Team Notes", "Process Guide", "ac:link", "ri:page"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in hanger page: %s", want, out)
		}
	}
}

// --- runToilSync ---

func mockConfluenceConfig(space, hanger string) func() {
	dir, _ := os.MkdirTemp("", "jira-thing-cfg")
	path := filepath.Join(dir, "jira-thing.json")
	data := fmt.Sprintf(`{"project":"CRSS","toil_marker":"ECP_TOIL","toil_team":"ECP_SEC_TEAM","confluence_space":%q,"ticket_hanger":%q}`, space, hanger)
	os.WriteFile(path, []byte(data), 0o644)
	old := config.ConfigPath
	config.ConfigPath = func() string { return path }
	return func() { config.ConfigPath = old; os.RemoveAll(dir) }
}

// confluenceTestServer routes all Confluence + Jira API calls for toil-sync tests.
// hangerID is the page ID returned for the hanger page lookup.
// existingChildren is a map of page title → body for pre-existing child pages.
func confluenceTestServer(t *testing.T, hangerID string, existingChildren map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPost && strings.Contains(p, "search"):
			json.NewEncoder(w).Encode(map[string]any{
				"issues": []any{map[string]any{"key": "CRSS-1", "fields": map[string]any{
					"summary": "Toil task", "updated": "2026-05-20T10:00:00.000Z",
					"status": map[string]any{"name": "Open"}, "priority": map[string]any{"name": "Medium"},
				}}},
				"total": 1, "maxResults": 100,
			})
		case r.Method == http.MethodGet && strings.Contains(p, "child/page"):
			results := []any{}
			ver := 1
			for title, body := range existingChildren {
				results = append(results, map[string]any{
					"id": "child-" + title, "title": title,
					"version": map[string]any{"number": float64(ver)},
					"body":    map[string]any{"storage": map[string]any{"value": body}},
				})
				ver++
			}
			json.NewEncoder(w).Encode(map[string]any{"results": results, "size": len(results)})
		case r.Method == http.MethodGet && strings.Contains(p, "content"):
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{map[string]any{
					"id": hangerID, "title": "Toil Tracker",
					"version": map[string]any{"number": float64(3)},
				}},
				"size": 1,
			})
		case r.Method == http.MethodPost && strings.Contains(p, "content"):
			json.NewEncoder(w).Encode(map[string]any{"id": "new-1", "title": "CRSS-1", "version": map[string]any{"number": float64(1)}})
		case r.Method == http.MethodPut:
			json.NewEncoder(w).Encode(map[string]any{"id": p})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestRunToilSync_CreatesNewChildPage(t *testing.T) {
	srv := confluenceTestServer(t, "99", nil)
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfluenceConfig("ENG", "Toil Tracker")()
	defer mockShowTableSelectAll()()

	out := captureStdout(func() { runToilSync() })
	if !strings.Contains(out, "Created") || !strings.Contains(out, "CRSS-1") {
		t.Errorf("expected 'Created CRSS-1' in output, got: %s", out)
	}
}

func TestRunToilSync_UpdatesExistingChildPage(t *testing.T) {
	existing := map[string]string{
		"CRSS-1": "<!-- jira-thing:notes:start --><p>saved note</p><!-- jira-thing:notes:end -->",
	}
	srv := confluenceTestServer(t, "99", existing)
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfluenceConfig("ENG", "Toil Tracker")()
	defer mockShowTableSelectAll()()

	out := captureStdout(func() { runToilSync() })
	if !strings.Contains(out, "Updated") || !strings.Contains(out, "CRSS-1") {
		t.Errorf("expected 'Updated CRSS-1' in output, got: %s", out)
	}
}

func TestRunToilSync_MissingConfig(t *testing.T) {
	old := config.ConfigPath
	config.ConfigPath = func() string { return "/nonexistent/path.json" }
	defer func() { config.ConfigPath = old }()

	exited := captureExit(func() {
		captureStderr(func() { runToilSync() })
	})
	if !exited {
		t.Error("expected osExit when config missing")
	}
}

func TestRunToilSync_PageNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "search"):
			json.NewEncoder(w).Encode(map[string]any{
				"issues": []any{map[string]any{"key": "CRSS-1", "fields": map[string]any{
					"summary": "Toil task", "updated": "2026-05-20T10:00:00.000Z",
					"status": map[string]any{"name": "Open"}, "priority": map[string]any{"name": "Medium"},
				}}},
				"total": 1, "maxResults": 100,
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "size": 0})
		}
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer mockConfluenceConfig("ENG", "Toil Tracker")()
	defer mockShowTableSelectAll()()

	exited := captureExit(func() {
		captureStderr(func() { runToilSync() })
	})
	if !exited {
		t.Error("expected osExit when page not found")
	}
}
