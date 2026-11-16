package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunList_JSONOutputsArray(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []map[string]any{
				{"key": "PROJ-1", "fields": map[string]any{"summary": "First", "status": map[string]any{"name": "Open"}}},
			},
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runList([]string{"-json"}) })
	var got []listedTicket
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(got) != 1 || got[0].Key != "PROJ-1" || got[0].Summary != "First" {
		t.Errorf("got %+v", got)
	}
	if got[0].URL != srv.URL+"/browse/PROJ-1" {
		t.Errorf("url = %q", got[0].URL)
	}
}

func TestRunList_JSONEmptyResultIsEmptyArray(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{}})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runList([]string{"-json"}) })
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("expected empty JSON array, got: %q", out)
	}
}

// listServer returns two issues and records the last search body.
func listServer(t *testing.T, body *map[string]any) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"isLast": true, "issues": []map[string]any{
			{"key": "PROJ-1", "fields": map[string]any{"summary": "=HYPERLINK(\"x\")", "status": map[string]any{"name": "Open"},
				"issuetype": map[string]any{"name": "Bug"}, "updated": "2026-11-16T09:00:00.000+0000"}},
			{"key": "PROJ-2", "fields": map[string]any{"summary": "Second, with comma", "status": map[string]any{"name": "Done"}}},
		}})
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
}

func TestRunList_Formats(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
		not  []string
	}{
		{"plain default columns", []string{"-plain"}, []string{"KEY", "TYPE", "PROJ-1", "Bug", "2026-11-16"}, nil},
		{"plain no headers", []string{"-plain", "-no-headers", "-columns", "key,status"}, []string{"PROJ-1  Open"}, []string{"KEY"}},
		{"columns alone is plain", []string{"-columns", "key"}, []string{"KEY\nPROJ-1\nPROJ-2"}, nil},
		{"csv", []string{"-csv", "-columns", "key,summary"}, []string{"KEY,SUMMARY", `PROJ-1,"'=HYPERLINK(""x"")"`, `PROJ-2,"Second, with comma"`}, nil},
		{"json columns", []string{"-json", "-columns", "key,status"}, []string{`{"key":"PROJ-1","status":"Open"}`}, []string{"summary"}},
		{"styled", nil, []string{"Found 2 ticket(s)", "PROJ-2"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			listServer(t, &body)
			out := captureStdout(func() { runList(tt.args) })
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, n := range tt.not {
				if strings.Contains(out, n) {
					t.Errorf("unexpected %q in:\n%s", n, out)
				}
			}
		})
	}
}

func TestRunList_NewFiltersAndLimit(t *testing.T) {
	var body map[string]any
	listServer(t, &body)
	captureStdout(func() {
		runList([]string{"-t", "Bug", "-R", "x", "-C", "api", "-created-before", "month", "-limit", "7", "-json"})
	})
	jql, _ := body["jql"].(string)
	for _, w := range []string{`issuetype = "Bug"`, "resolution is EMPTY", `component in ("api")`, "created < startOfMonth()"} {
		if !strings.Contains(jql, w) {
			t.Errorf("missing %q in %q", w, jql)
		}
	}
	if body["maxResults"] != 7.0 {
		t.Errorf("maxResults = %v", body["maxResults"])
	}
}

func TestRunList_Rejects(t *testing.T) {
	for _, args := range [][]string{{"-json", "-csv"}, {"-columns", "bogus"}, {"-limit", "5000"}, {"stray"}, {"-order-by", "a b"}} {
		var body map[string]any
		listServer(t, &body)
		captureStderr(func() {
			if !captureExit(func() { runList(args) }) {
				t.Errorf("%v: expected exit", args)
			}
		})
	}
}

func TestCSVSafe(t *testing.T) {
	for in, want := range map[string]string{"=1+1": "'=1+1", "+x": "'+x", "-2": "'-2", "@a": "'@a", "plain": "plain", "": ""} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q", in, got)
		}
	}
}
