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

func TestListEpicsAndIssues(t *testing.T) {
	var jqls []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		jql, _ := body["jql"].(string)
		jqls = append(jqls, jql)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{{"key": "P-1"}}})
	}))
	t.Cleanup(srv.Close)
	conn := api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
	epics, err := ListEpics(conn, "PROJ")
	if err != nil || len(epics) != 1 || !strings.Contains(jqls[0], "issuetype = Epic") {
		t.Errorf("epics=%v jql=%v err=%v", epics, jqls, err)
	}
	issues, err := ListEpicIssues(conn, "P-1")
	if err != nil || len(issues) != 1 || len(jqls) != 2 || jqls[1] != `parent = "P-1"` {
		t.Errorf("issues=%v jqls=%v err=%v", issues, jqls, err)
	}
}

func TestListEpics_Rejects(t *testing.T) {
	if _, err := ListEpics(api.JiraConnection{}, ""); !errors.Is(err, ErrProjectRequired) {
		t.Errorf("empty: %v", err)
	}
	if _, err := ListEpics(api.JiraConnection{}, "../x"); err == nil {
		t.Error("expected a bad key error")
	}
}
