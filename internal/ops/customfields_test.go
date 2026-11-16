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

// fieldRegistry is a field list covering each supported type.
var fieldRegistry = []map[string]any{
	{"id": "summary", "name": "Summary", "schema": map[string]any{"type": "string"}},
	{"id": "customfield_1", "name": "Story Points", "schema": map[string]any{"type": "number"}},
	{"id": "customfield_2", "name": "Team Colour", "schema": map[string]any{"type": "option"}},
	{"id": "customfield_3", "name": "Reviewers", "schema": map[string]any{"type": "array", "items": "user"}},
	{"id": "customfield_4", "name": "Due Soon", "schema": map[string]any{"type": "date"}},
	{"id": "customfield_5", "name": "Notes", "schema": map[string]any{"type": "string", "custom": customTextarea}},
	{"id": "customfield_6", "name": "Sprint", "schema": map[string]any{"type": "array", "items": "json", "custom": customSprint}},
	{"id": "customfield_7", "name": "Tags", "schema": map[string]any{"type": "array", "items": "string"}},
	{"id": "customfield_8", "name": "Dup", "schema": map[string]any{"type": "string"}},
	{"id": "customfield_9", "name": "dup", "schema": map[string]any{"type": "string"}},
	{"id": "customfield_10", "name": "Odd", "schema": map[string]any{"type": "any"}},
	{"id": "customfield_11", "name": "Fix In", "schema": map[string]any{"type": "array", "items": "version"}},
}

// fieldJira serves the field registry, myself and user search.
func fieldJira(t *testing.T) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/field"):
			_ = json.NewEncoder(w).Encode(fieldRegistry)
		case strings.HasSuffix(r.URL.Path, "/myself"):
			_ = json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me"})
		case strings.HasSuffix(r.URL.Path, "/user/search"):
			_ = json.NewEncoder(w).Encode([]map[string]any{human("a1", "Ada", "ada@x.io")})
		}
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func TestParseCustom(t *testing.T) {
	tests := []struct {
		in      string
		want    CustomValue
		wantErr bool
	}{
		{"Story Points=3", CustomValue{"Story Points", "3"}, false},
		{" Team = Platform ", CustomValue{"Team", "Platform"}, false},
		{"Formula=a=b", CustomValue{"Formula", "a=b"}, false},
		{"Clear=", CustomValue{"Clear", ""}, false},
		{"novalue", CustomValue{}, true},
		{"=x", CustomValue{}, true},
	}
	for _, tt := range tests {
		got, err := ParseCustom(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseCustom(%q) = %+v, %v", tt.in, got, err)
		}
	}
}

func TestResolveCustomFields_Types(t *testing.T) {
	conn := fieldJira(t)
	got, err := ResolveCustomFields(conn, []CustomValue{
		{"story points", "2.5"}, {"Team Colour", "Red"}, {"Reviewers", "me, ada"}, {"Due Soon", "2026-12-31"},
		{"Notes", "**bold**"}, {"Sprint", "42"}, {"Tags", "a, b"}, {"customfield_8", "by id"}, {"Fix In", "v1"},
		{"Summary", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(got)
	for _, want := range []string{
		`"customfield_1":2.5`, `"customfield_2":{"value":"Red"}`,
		`"customfield_3":[{"accountId":"acc-me"},{"accountId":"a1"}]`, `"customfield_4":"2026-12-31"`,
		`"customfield_5":{"content"`, `"customfield_6":42`, `"customfield_7":["a","b"]`, `"customfield_8":"by id"`,
		`"customfield_11":[{"name":"v1"}]`, `"summary":null`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
}

func TestResolveCustomFields_Errors(t *testing.T) {
	conn := fieldJira(t)
	tests := []struct {
		name  string
		value CustomValue
		check func(error) bool
	}{
		{"unknown", CustomValue{"Nope", "1"}, func(err error) bool { return errors.Is(err, ErrUnknownField) }},
		{"ambiguous", CustomValue{"DUP", "x"}, func(err error) bool {
			var a *AmbiguousFieldError
			return errors.As(err, &a) && strings.Contains(err.Error(), "customfield_8") && strings.Contains(err.Error(), "customfield_9")
		}},
		{"not a number", CustomValue{"Story Points", "lots"}, func(err error) bool { return strings.Contains(err.Error(), "Story Points (customfield_1)") }},
		{"bad date", CustomValue{"Due Soon", "31/12/2026"}, func(err error) bool { return strings.Contains(err.Error(), "not a date") }},
		{"bad sprint", CustomValue{"Sprint", "next"}, func(err error) bool { return strings.Contains(err.Error(), "sprint ID") }},
		{"unsupported", CustomValue{"Odd", "x"}, func(err error) bool { return strings.Contains(err.Error(), `"any" isn't supported`) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ResolveCustomFields(conn, []CustomValue{tt.value}); !tt.check(err) {
				t.Errorf("err = %v", err)
			}
		})
	}
	if got, err := ResolveCustomFields(api.JiraConnection{}, nil); got != nil || err != nil {
		t.Errorf("no values: got=%v err=%v", got, err)
	}
}

func TestCoerceArray_UnsupportedItems(t *testing.T) {
	if _, err := coerceArray(api.JiraConnection{}, "json", "x"); err == nil || !strings.Contains(err.Error(), "list of") {
		t.Errorf("err = %v", err)
	}
}

func TestCreateAndUpdate_CarryCustomFields(t *testing.T) {
	conn := fieldJira(t)
	created, err := CreateTicket(conn, CreateRequest{
		Project: "P", Summary: "S", Custom: []CustomValue{{"Story Points", "3"}},
	}, CreateOptions{DryRun: true})
	if err != nil || created.Payload["customfield_1"] != 3.0 {
		t.Errorf("create payload=%v err=%v", created.Payload, err)
	}
	updated, err := UpdateTicket(conn, UpdateRequest{Key: "K-1", Custom: []CustomValue{{"Team Colour", "Blue"}}}, true)
	if err != nil || nestedStr(updated.Fields, "customfield_2", "value") != "Blue" {
		t.Errorf("update fields=%v err=%v", updated.Fields, err)
	}
	if _, err := CreateTicket(conn, CreateRequest{Project: "P", Summary: "S", Custom: []CustomValue{{"Nope", "1"}}}, CreateOptions{DryRun: true}); !errors.Is(err, ErrUnknownField) {
		t.Errorf("create unknown field: err=%v", err)
	}
}

func TestCreateTicket_FixVersions(t *testing.T) {
	res, err := CreateTicket(fieldJira(t), CreateRequest{Project: "P", Summary: "S", FixVersions: []string{"v1", "v2"}}, CreateOptions{DryRun: true})
	body, _ := json.Marshal(res.Payload["fixVersions"])
	if err != nil || string(body) != `[{"name":"v1"},{"name":"v2"}]` {
		t.Errorf("fixVersions=%s err=%v", body, err)
	}
}
