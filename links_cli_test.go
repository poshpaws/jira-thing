package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// linkRequests records the mutating calls linksHandler received.
type linkRequests struct {
	linkBody   map[string]any
	remoteBody map[string]any
	deletedID  string
}

// linksHandler fakes the Jira link endpoints: link types, issue-link create/delete,
// the issuelinks lookup UnlinkIssues performs, and remote-link create.
func linksHandler(t *testing.T, rec *linkRequests) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/issueLinkType"):
			json.NewEncoder(w).Encode(map[string]any{"issueLinkTypes": []map[string]any{
				{"id": "1", "name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
				{"id": "2", "name": "Relates", "inward": "relates to", "outward": "relates to"},
			}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issueLink"):
			json.NewDecoder(r.Body).Decode(&rec.linkBody)
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/issue/PROJ-1"):
			json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{"issuelinks": []map[string]any{
				{"id": "555", "outwardIssue": map[string]any{"key": "PROJ-2"}},
			}}})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/issueLink/"):
			rec.deletedID = r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/remotelink"):
			json.NewDecoder(r.Body).Decode(&rec.remoteBody)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}
}

// startLinksServer starts a fake Jira and points credentials at it; call the
// returned func to tear both down.
func startLinksServer(t *testing.T) (*linkRequests, func()) {
	rec := &linkRequests{}
	srv := httptest.NewTLSServer(linksHandler(t, rec))
	restore := mockCreds(srv.URL)
	return rec, func() { restore(); srv.Close() }
}

func TestRunLinkTypes_ListsLiveTypes(t *testing.T) {
	_, done := startLinksServer(t)
	defer done()

	out := captureStdout(func() { runLinkTypes(nil) })
	if !strings.Contains(out, "Blocks") || !strings.Contains(out, "is blocked by") || !strings.Contains(out, "Relates") {
		t.Errorf("expected live link types listed, got: %s", out)
	}
}

func TestRunLinkTypes_JSON(t *testing.T) {
	_, done := startLinksServer(t)
	defer done()

	out := captureStdout(func() { runLinkTypes([]string{"-json"}) })
	var types []map[string]any
	if err := json.Unmarshal([]byte(out), &types); err != nil {
		t.Fatalf("expected JSON array, got %q: %v", out, err)
	}
	if len(types) != 2 || types[0]["name"] != "Blocks" || types[0]["outward"] != "blocks" {
		t.Errorf("unexpected JSON: %v", types)
	}
}

func TestRunLink_CreatesLinkInGivenDirection(t *testing.T) {
	rec, done := startLinksServer(t)
	defer done()

	out := captureStdout(func() { runLink([]string{"PROJ-1", "PROJ-2", "-type", "blocks"}) })
	outward, _ := rec.linkBody["outwardIssue"].(map[string]any)
	inward, _ := rec.linkBody["inwardIssue"].(map[string]any)
	linkType, _ := rec.linkBody["type"].(map[string]any)
	if outward["key"] != "PROJ-1" || inward["key"] != "PROJ-2" {
		t.Errorf("wrong direction: %v", rec.linkBody)
	}
	if linkType["name"] != "Blocks" {
		t.Errorf("expected canonical type name Blocks, got %v", linkType["name"])
	}
	if !strings.Contains(out, "PROJ-1") || !strings.Contains(out, "blocks") || !strings.Contains(out, "PROJ-2") {
		t.Errorf("expected success message stating direction, got: %s", out)
	}
}

func TestRunLink_JSON(t *testing.T) {
	_, done := startLinksServer(t)
	defer done()

	out := captureStdout(func() { runLink([]string{"PROJ-1", "PROJ-2", "-type", "Relates", "-json"}) })
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("expected JSON, got %q: %v", out, err)
	}
	if got["outward"] != "PROJ-1" || got["inward"] != "PROJ-2" || got["type"] != "Relates" || got["status"] != "linked" {
		t.Errorf("unexpected JSON: %v", got)
	}
}

func TestRunLink_MissingOrUnknownTypeListsTypes(t *testing.T) {
	cases := map[string][]string{
		"omitted": {"PROJ-1", "PROJ-2"},
		"unknown": {"PROJ-1", "PROJ-2", "-type", "Bogus"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			rec, done := startLinksServer(t)
			defer done()

			out := captureStderr(func() {
				if !captureExit(func() { runLink(args) }) {
					t.Error("expected osExit")
				}
			})
			if !strings.Contains(out, "Blocks") || !strings.Contains(out, "Relates") {
				t.Errorf("expected live types listed, got: %s", out)
			}
			if rec.linkBody != nil {
				t.Errorf("no link should be created, got %v", rec.linkBody)
			}
		})
	}
}

func TestRunLink_DryRunMakesNoCall(t *testing.T) {
	rec, done := startLinksServer(t)
	defer done()

	out := captureStdout(func() { runLink([]string{"PROJ-1", "PROJ-2", "-type", "Blocks", "-dry-run"}) })
	if rec.linkBody != nil {
		t.Errorf("dry run must not POST, got %v", rec.linkBody)
	}
	if !strings.Contains(out, "Dry run") || !strings.Contains(out, "PROJ-2") {
		t.Errorf("expected dry-run payload, got: %s", out)
	}
}

func TestRunUnlink_DeletesLinkBetweenIssues(t *testing.T) {
	rec, done := startLinksServer(t)
	defer done()

	out := captureStdout(func() { runUnlink([]string{"PROJ-1", "PROJ-2"}) })
	if rec.deletedID != "555" {
		t.Errorf("expected link 555 deleted, got %q", rec.deletedID)
	}
	if !strings.Contains(out, "Unlinked") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestRunRemoteLink_DefaultsTitleToURL(t *testing.T) {
	rec, done := startLinksServer(t)
	defer done()

	captureStdout(func() { runRemoteLink([]string{"PROJ-1", "-url", "https://example.com/doc"}) })
	object, _ := rec.remoteBody["object"].(map[string]any)
	if object["url"] != "https://example.com/doc" || object["title"] != "https://example.com/doc" {
		t.Errorf("unexpected remote link payload: %v", rec.remoteBody)
	}
}

func TestRunLinkCommands_UsageErrors(t *testing.T) {
	cases := map[string]func(){
		"link one key":       func() { runLink([]string{"PROJ-1"}) },
		"unlink one key":     func() { runUnlink([]string{"PROJ-1"}) },
		"remote-link no url": func() { runRemoteLink([]string{"PROJ-1"}) },
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			out := captureStderr(func() {
				if !captureExit(run) {
					t.Error("expected osExit")
				}
			})
			if !strings.Contains(out, "usage:") {
				t.Errorf("expected usage message, got: %s", out)
			}
		})
	}
}
