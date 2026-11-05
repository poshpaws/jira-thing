package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

// searchServer fakes the search endpoint, returning one issue per summary
// (keyed PROJ-1, PROJ-2, ...) and recording the JQL it was sent.
func searchServer(t *testing.T, jql *string, summaries ...string) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, api.SearchEndpoint) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*jql, _ = body["jql"].(string)
		issues := make([]map[string]any, len(summaries))
		for i, s := range summaries {
			issues[i] = map[string]any{"key": fmt.Sprintf("PROJ-%d", i+1), "fields": map[string]any{"summary": s}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"issues": issues})
	}))
}

func TestFindOpenIssueBySummary_Matching(t *testing.T) {
	cases := []struct {
		name  string
		found []string
		want  string
	}{
		{"exact match", []string{"Fix login"}, "PROJ-1"},
		{"case and whitespace ignored", []string{"  fix LOGIN "}, "PROJ-1"},
		{"fuzzy hit skipped for exact one", []string{"Fix login page", "Fix login"}, "PROJ-2"},
		{"fuzzy hit only", []string{"Fix login page"}, ""},
		{"nothing found", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var jql string
			srv := searchServer(t, &jql, tc.found...)
			defer srv.Close()

			got, err := api.FindOpenIssueBySummary(conn(srv.URL), "PROJ", "Fix login")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFindOpenIssueBySummary_JQLScopesAndEscapes(t *testing.T) {
	var jql string
	srv := searchServer(t, &jql)
	defer srv.Close()

	if _, err := api.FindOpenIssueBySummary(conn(srv.URL), "PROJ", `Say "hi" \ now`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		`project = "PROJ"`,
		`statusCategory != Done`,
		`summary ~ "\"Say \\\"hi\\\" \\\\ now\""`,
	} {
		if !strings.Contains(jql, want) {
			t.Errorf("JQL %s missing %s", jql, want)
		}
	}
}

func TestFindOpenIssueBySummary_SearchFailureIsError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := api.FindOpenIssueBySummary(conn(srv.URL), "PROJ", "S"); err == nil {
		t.Error("expected error when the search fails")
	}
}

func TestFindOpenIssueBySummary_TruncatedResultsAreError(t *testing.T) {
	var jql string
	near := make([]string, api.DedupeMaxResults)
	for i := range near {
		near[i] = "Fix login page"
	}
	srv := searchServer(t, &jql, near...)
	defer srv.Close()

	if _, err := api.FindOpenIssueBySummary(conn(srv.URL), "PROJ", "Fix login"); err == nil {
		t.Error("expected error when a full page leaves the check inconclusive")
	}
}
