package ops

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/template"
)

func init() {
	api.SetHTTPClient(&http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- test only
	}})
}

// fakeJira records requests and answers the endpoints ops calls.
type fakeJira struct {
	existing   map[string]string // summary -> open ticket key
	searchFail bool
	linkFail   bool
	created    []map[string]any
	updates    []map[string]any
	links      int
}

func (f *fakeJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	switch {
	case strings.HasSuffix(r.URL.Path, "/myself"):
		_ = json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me"})
	case strings.HasSuffix(r.URL.Path, "/search/jql"):
		f.serveSearch(w)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issue"):
		f.created = append(f.created, body["fields"].(map[string]any))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "NEW-1"})
	case r.Method == http.MethodPut:
		f.serveUpdate(w, body)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (f *fakeJira) serveSearch(w http.ResponseWriter) {
	if f.searchFail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	issues := []map[string]any{}
	for summary, key := range f.existing {
		issues = append(issues, map[string]any{"key": key, "fields": map[string]any{"summary": summary}})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"issues": issues})
}

// serveUpdate treats a PUT setting "parent" as an epic/parent link, and any
// other PUT as a field update.
func (f *fakeJira) serveUpdate(w http.ResponseWriter, body map[string]any) {
	fields, _ := body["fields"].(map[string]any)
	if _, isParent := fields["parent"]; !isParent {
		f.updates = append(f.updates, fields)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	f.links++
	if f.linkFail {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func startFake(t *testing.T, f *fakeJira) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(f)
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func TestCreateTicket_TypedFieldsOverrideBase(t *testing.T) {
	f := &fakeJira{}
	conn := startFake(t, f)
	res, err := CreateTicket(conn, CreateRequest{
		Fields:  map[string]any{"project": "OLD", "summary": "base", "customfield_1": "x"},
		Project: "PROJ", Summary: "Fix", Labels: []string{"a"}, Components: []string{"api"}, Assignee: "me",
	}, CreateOptions{})
	if err != nil || res.Status != StatusCreated || res.Key != "NEW-1" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	got := f.created[0]
	if nestedStr(got, "project", "key") != "PROJ" || got["summary"] != "Fix" || nestedStr(got, "issuetype", "name") != DefaultIssueType {
		t.Errorf("unexpected payload %v", got)
	}
	if _, ok := got["customfield_1"]; ok || nestedStr(got, "assignee", "accountId") != "acc-me" {
		t.Errorf("custom field not stripped or assignee unresolved: %v", got)
	}
}

func TestCreateTicket_MissingSummaryErrors(t *testing.T) {
	_, err := CreateTicket(api.JiraConnection{}, CreateRequest{Project: "PROJ"}, CreateOptions{})
	if !errors.Is(err, ErrSummaryRequired) {
		t.Errorf("expected ErrSummaryRequired, got %v", err)
	}
}

func TestCreateTicket_DedupeReportsExisting(t *testing.T) {
	f := &fakeJira{existing: map[string]string{"Fix": "PROJ-7"}}
	conn := startFake(t, f)
	res, err := CreateTicket(conn, CreateRequest{Project: "PROJ", Summary: "fix "},
		CreateOptions{Dedupe: NewDedupeGuard(conn, true)})
	if err != nil || res.Status != StatusExists || res.Key != "PROJ-7" || len(f.created) != 0 {
		t.Errorf("res=%+v err=%v created=%d", res, err, len(f.created))
	}
}

func TestCreateTicket_DedupeSearchFailureCreatesNothing(t *testing.T) {
	f := &fakeJira{searchFail: true}
	conn := startFake(t, f)
	_, err := CreateTicket(conn, CreateRequest{Project: "PROJ", Summary: "S"},
		CreateOptions{Dedupe: NewDedupeGuard(conn, true)})
	if err == nil || !strings.Contains(err.Error(), "nothing created") || len(f.created) != 0 {
		t.Errorf("err=%v created=%d", err, len(f.created))
	}
}

func TestCreateTicket_DedupeWithoutProjectErrors(t *testing.T) {
	_, err := CreateTicket(api.JiraConnection{}, CreateRequest{Summary: "S"},
		CreateOptions{Dedupe: NewDedupeGuard(api.JiraConnection{}, true)})
	if !errors.Is(err, ErrDedupeNeedsProject) {
		t.Errorf("expected ErrDedupeNeedsProject, got %v", err)
	}
}

func TestCreateTicket_DryRunSendsNothingAndRemembers(t *testing.T) {
	f := &fakeJira{}
	conn := startFake(t, f)
	guard := NewDedupeGuard(conn, true)
	opts := CreateOptions{Dedupe: guard, DryRun: true, Label: "ticket 1 in this batch"}
	res, err := CreateTicket(conn, CreateRequest{Project: "PROJ", Summary: "S", Parent: "PROJ-1"}, opts)
	if err != nil || res.Status != StatusDryRun || res.Parent != "PROJ-1" || len(f.created) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if _, ok := res.Payload["parent"]; ok {
		t.Errorf("parent must not be in the POST payload: %v", res.Payload)
	}
	again, _ := CreateTicket(conn, CreateRequest{Project: "PROJ", Summary: "s"}, opts)
	if again.Status != StatusExists || again.Key != "ticket 1 in this batch" {
		t.Errorf("expected repeat flagged, got %+v", again)
	}
}

func TestCreateTicket_ParentLinkFailureReportedNotFatal(t *testing.T) {
	f := &fakeJira{linkFail: true}
	conn := startFake(t, f)
	res, err := CreateTicket(conn, CreateRequest{Project: "PROJ", Summary: "S", Parent: "PROJ-1"}, CreateOptions{})
	if err != nil || res.Key != "NEW-1" || res.ParentErr == nil {
		t.Errorf("res=%+v err=%v", res, err)
	}
}

func TestPreparePayload_ResolvesSelfMarker(t *testing.T) {
	conn := startFake(t, &fakeJira{})
	payload, _, err := PreparePayload(conn, map[string]any{"assignee": template.AssigneeSelf})
	if err != nil || nestedStr(payload, "assignee", "accountId") != "acc-me" {
		t.Errorf("payload=%v err=%v", payload, err)
	}
}

func TestUpdateTicket_NothingToUpdate(t *testing.T) {
	if _, err := UpdateTicket(api.JiraConnection{}, UpdateRequest{Key: "K-1"}, false); !errors.Is(err, ErrNothingToUpdate) {
		t.Errorf("expected ErrNothingToUpdate, got %v", err)
	}
}

func TestUpdateTicket_SendsOnlyGivenFields(t *testing.T) {
	f := &fakeJira{}
	conn := startFake(t, f)
	res, err := UpdateTicket(conn, UpdateRequest{Key: "K-1", Priority: "High", Description: "# Hi"}, false)
	if err != nil || res.Status != StatusEdited || len(f.updates) != 1 {
		t.Fatalf("res=%+v err=%v updates=%v", res, err, f.updates)
	}
	if _, ok := f.updates[0]["summary"]; ok || nestedStr(f.updates[0], "priority", "name") != "High" {
		t.Errorf("unexpected fields %v", f.updates[0])
	}
}

func TestUpdateTicket_DryRunSendsNothing(t *testing.T) {
	f := &fakeJira{}
	conn := startFake(t, f)
	res, err := UpdateTicket(conn, UpdateRequest{Key: "K-1", Summary: "S", Parent: "P-1"}, true)
	if err != nil || res.Status != StatusDryRun || len(f.updates) != 0 || f.links != 0 {
		t.Errorf("res=%+v err=%v", res, err)
	}
}

func TestResolveAssignee(t *testing.T) {
	conn := startFake(t, &fakeJira{})
	tests := []struct {
		arg  string
		want any
	}{
		{AssigneeUnassign, nil},
		{AssigneeMe, "acc-me"},
		{"5b10ac8d82e05b22cc7d4ef5", "5b10ac8d82e05b22cc7d4ef5"},
		{"557058:f58131cb-b67d-43c7-b30d-6b58d40bd077", "557058:f58131cb-b67d-43c7-b30d-6b58d40bd077"},
	}
	for _, tt := range tests {
		got, err := ResolveAssignee(conn, tt.arg)
		if err != nil {
			t.Fatalf("%s: %v", tt.arg, err)
		}
		if tt.want == nil && got != nil {
			t.Errorf("%s: got %v, want nil", tt.arg, got)
		}
		if m, _ := got.(map[string]any); tt.want != nil && m["accountId"] != tt.want {
			t.Errorf("%s: got %v", tt.arg, got)
		}
	}
}
