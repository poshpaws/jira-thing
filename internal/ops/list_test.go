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

func TestBuildListJQL(t *testing.T) {
	tests := []struct {
		name   string
		filter ListFilter
		want   []string
	}{
		{"empty", ListFilter{}, []string{"ORDER BY created DESC"}},
		{"assignee me", ListFilter{Assignee: "me"}, []string{"assignee = currentUser()"}},
		{"not me", ListFilter{Assignee: "~me"}, []string{"assignee != currentUser()"}},
		{"unassigned", ListFilter{Assignee: "x"}, []string{"assignee is EMPTY"}},
		{"assigned", ListFilter{Assignee: "~x"}, []string{"assignee is not EMPTY"}},
		{"named", ListFilter{Assignee: "Ada"}, []string{`assignee = "Ada"`}},
		{"not named", ListFilter{Assignee: "~Ada"}, []string{`assignee != "Ada"`}},
		{"reporter me", ListFilter{Reporter: "me"}, []string{"reporter = currentUser()"}},
		{"priority status", ListFilter{Priority: "High", Status: "~Done"}, []string{`priority = "High"`, `status != "Done"`}},
		{"type", ListFilter{Type: "Bug"}, []string{`issuetype = "Bug"`}},
		{"unresolved", ListFilter{Resolution: "x"}, []string{"resolution is EMPTY"}},
		{"project", ListFilter{Project: "PROJ"}, []string{`project = "PROJ"`}},
		{"components", ListFilter{Components: []string{"api", "web"}}, []string{`component in ("api", "web")`}},
		{"labels", ListFilter{Labels: []string{"backend"}}, []string{`labels in ("backend")`}},
		{"created relative", ListFilter{Created: "-7d"}, []string{`created >= "-7d"`}},
		{"created keyword", ListFilter{Created: "week"}, []string{"created >= startOfWeek()"}},
		{"created before", ListFilter{CreatedBefore: "2026-01-01"}, []string{`created < "2026-01-01"`}},
		{"before keyword", ListFilter{CreatedBefore: "month"}, []string{"created < startOfMonth()"}},
		{"updated", ListFilter{Updated: "-30m"}, []string{`updated >= "-30m"`}},
		{"watching", ListFilter{Watching: true}, []string{"watcher = currentUser()"}},
		{"raw jql", ListFilter{JQL: `summary ~ "cli"`}, []string{`(summary ~ "cli")`}},
		{"order reverse", ListFilter{OrderBy: "cf[10020]", Reverse: true}, []string{"ORDER BY cf[10020] ASC"}},
		{"quotes escaped", ListFilter{Status: `a" OR project = X OR status = "b`}, []string{`status = "a\" OR project = X OR status = \"b"`}},
		{"date injection quoted", ListFilter{Created: "-1d OR project = X"}, []string{`created >= "-1d OR project = X"`}},
		{"combined", ListFilter{Assignee: "me", Priority: "High"}, []string{`assignee = currentUser() AND priority = "High" ORDER BY created DESC`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildListJQL(tt.filter)
			for _, w := range tt.want {
				if err != nil || !strings.Contains(got, w) {
					t.Errorf("got %q (err %v), want %q", got, err, w)
				}
			}
		})
	}
}

func TestBuildListJQL_RawWithOrderByPassesThrough(t *testing.T) {
	raw := "project = P order by rank"
	if got, _ := BuildListJQL(ListFilter{JQL: raw}); got != raw {
		t.Errorf("got %q", got)
	}
	if got, _ := BuildListJQL(ListFilter{JQL: raw, Type: "Bug"}); got == raw {
		t.Errorf("with a filter: got %q", got)
	}
	if got, _ := BuildListJQL(ListFilter{JQL: "project = P"}); got != "(project = P) ORDER BY created DESC" {
		t.Errorf("no order by: got %q", got)
	}
}

func TestBuildListJQL_RejectsBadOrderBy(t *testing.T) {
	for _, bad := range []string{"created; DROP", "created DESC, key", "(x)"} {
		if _, err := BuildListJQL(ListFilter{OrderBy: bad}); !errors.Is(err, ErrBadOrderBy) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestParseColumns(t *testing.T) {
	cols, err := ParseColumns("")
	if err != nil || strings.Join(cols, ",") != strings.Join(DefaultColumns, ",") {
		t.Errorf("default: cols=%v err=%v", cols, err)
	}
	cols, err = ParseColumns(" Key, summary ")
	if err != nil || strings.Join(cols, ",") != "key,summary" {
		t.Errorf("cols=%v err=%v", cols, err)
	}
	if _, err := ParseColumns("key,bogus"); err == nil || !strings.Contains(err.Error(), "components") {
		t.Errorf("err = %v", err)
	}
}

func TestTicketRow(t *testing.T) {
	issue := map[string]any{"key": "P-1", "fields": map[string]any{
		"summary": "Fix\nit", "status": map[string]any{"name": "Open"}, "labels": []any{"a", "b"},
		"components": []any{map[string]any{"name": "api"}}, "updated": "2026-11-16T09:00:00.000+0000",
		"assignee": nil,
	}}
	conn := api.JiraConnection{BaseURL: "https://x.atlassian.net"}
	got := TicketRow(conn, issue, []string{"key", "url", "summary", "status", "labels", "components", "updated", "assignee"})
	want := "P-1|https://x.atlassian.net/browse/P-1|Fix it|Open|a,b|api|2026-11-16|"
	if strings.Join(got, "|") != want {
		t.Errorf("row = %q", strings.Join(got, "|"))
	}
	if got := TicketRow(conn, map[string]any{"key": "P-2"}, []string{"summary"}); got[0] != "" {
		t.Errorf("no fields: %q", got)
	}
}

func TestListTickets(t *testing.T) {
	var body map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"isLast": true, "issues": []map[string]any{{"key": "P-1"}}})
	}))
	t.Cleanup(srv.Close)
	conn := api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
	res, err := ListTickets(conn, ListRequest{Filter: ListFilter{Type: "Bug"}, Columns: []string{"key", "assignee", "status"}, Limit: 5})
	if err != nil || len(res.Issues) != 1 || !strings.Contains(res.JQL, "issuetype") {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	fieldsSent, _ := json.Marshal(body["fields"])
	if string(fieldsSent) != `["summary","status","priority","updated","assignee"]` || body["maxResults"] != 5.0 {
		t.Errorf("body = %v", body)
	}
}

func TestListTickets_Rejects(t *testing.T) {
	for _, req := range []ListRequest{{Limit: -1}, {Limit: MaxListLimit + 1}, {Filter: ListFilter{OrderBy: "x y"}}} {
		if _, err := ListTickets(api.JiraConnection{}, req); err == nil {
			t.Errorf("%+v: expected error", req)
		}
	}
}
