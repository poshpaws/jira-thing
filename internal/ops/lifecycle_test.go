package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

// lifecycleJira fakes the issue, transition, delete and worklog endpoints,
// recording each mutating request's path and body.
type lifecycleJira struct {
	source    map[string]any
	requests  []string
	bodies    []map[string]any
	transFail bool
}

func (j *lifecycleJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if r.Method != http.MethodGet {
		j.requests = append(j.requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		j.bodies = append(j.bodies, body)
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/myself"):
		_ = json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me", "displayName": "Me", "emailAddress": "me@x.io"})
	case strings.HasSuffix(r.URL.Path, "/transitions") && r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{{"id": "31", "name": "In Progress"}}})
	case strings.HasSuffix(r.URL.Path, "/transitions") && j.transFail:
		w.WriteHeader(http.StatusBadRequest)
	case r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "SRC-1", "fields": j.source})
	case strings.HasSuffix(r.URL.Path, "/issue"):
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "NEW-2"})
	case strings.HasSuffix(r.URL.Path, "/worklog"):
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "1"})
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func startLifecycle(t *testing.T, j *lifecycleJira) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(j)
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func TestTransitionTicket_MatchesStateCaseInsensitively(t *testing.T) {
	j := &lifecycleJira{}
	conn := startLifecycle(t, j)
	res, err := TransitionTicket(conn, TransitionRequest{Key: "K-1", State: "in progress", Resolution: "Fixed", Comment: "done"}, false)
	if err != nil || res.Status != StatusMoved || res.Transition.ID != "31" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	fields, _ := j.bodies[0]["fields"].(map[string]any)
	if nestedStr(fields, "resolution", "name") != "Fixed" || j.bodies[0]["update"] == nil {
		t.Errorf("resolution or comment missing: %v", j.bodies[0])
	}
}

func TestTransitionTicket_UnknownStateListsAvailable(t *testing.T) {
	conn := startLifecycle(t, &lifecycleJira{})
	_, err := TransitionTicket(conn, TransitionRequest{Key: "K-1", State: "Done"}, false)
	var terr *TransitionError
	if !errors.As(err, &terr) || !strings.Contains(err.Error(), "- In Progress") {
		t.Errorf("expected TransitionError listing states, got %v", err)
	}
}

func TestTransitionTicket_PickedTransitionSkipsLookupAndDryRunSendsNothing(t *testing.T) {
	j := &lifecycleJira{transFail: true}
	conn := startLifecycle(t, j)
	picked := api.Transition{ID: "99", Name: "Closed"}
	res, err := TransitionTicket(conn, TransitionRequest{Key: "K-1", Transition: picked}, true)
	if err != nil || res.Status != StatusDryRun || len(j.requests) != 0 {
		t.Errorf("res=%+v err=%v requests=%v", res, err, j.requests)
	}
	if nestedStr(res.Payload, "transition", "id") != "99" {
		t.Errorf("payload=%v", res.Payload)
	}
}

func TestTransitionTicket_APIFailureWrapped(t *testing.T) {
	conn := startLifecycle(t, &lifecycleJira{transFail: true})
	_, err := TransitionTicket(conn, TransitionRequest{Key: "K-1", State: "In Progress"}, false)
	if err == nil || !strings.Contains(err.Error(), "transitioning K-1") {
		t.Errorf("got %v", err)
	}
}

func TestCloneTicket_InheritsAndPrefixesSummary(t *testing.T) {
	j := &lifecycleJira{source: map[string]any{
		"project": map[string]any{"key": "PROJ"}, "issuetype": map[string]any{"name": "Bug"},
		"summary": "Broken", "status": map[string]any{"name": "Done"}, "labels": []any{"x"},
	}}
	conn := startLifecycle(t, j)
	res, err := CloneTicket(conn, CloneRequest{SourceKey: "SRC-1", Assignee: "me"}, false)
	if err != nil || res.Key != "NEW-2" || res.Status != StatusCloned {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	fields, _ := j.bodies[0]["fields"].(map[string]any)
	if fields["summary"] != ClonePrefix+"Broken" || nestedStr(fields, "issuetype", "name") != "Bug" {
		t.Errorf("unexpected clone payload %v", fields)
	}
	if _, ok := fields["status"]; ok || nestedStr(fields, "assignee", "accountId") != "acc-me" {
		t.Errorf("status copied or assignee unresolved: %v", fields)
	}
}

func TestWhoAmI(t *testing.T) {
	conn := startLifecycle(t, &lifecycleJira{})
	me, err := WhoAmI(conn)
	if err != nil || me != (User{AccountID: "acc-me", DisplayName: "Me", Email: "me@x.io"}) {
		t.Errorf("me=%+v err=%v", me, err)
	}
}
