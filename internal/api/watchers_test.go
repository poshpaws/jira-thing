package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"jira-thing/internal/api"
)

func TestWatchers(t *testing.T) {
	var method, body, query string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/watchers" {
			t.Errorf("path = %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		method, body, query = r.Method, string(b), r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]any{"isWatching": true, "watchCount": 1,
				"watchers": []map[string]any{{"accountId": "a1", "displayName": "Ada"}}})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := conn(srv.URL)
	got, err := api.FetchWatchers(c, "PROJ-1")
	if err != nil || !got.IsWatching || len(got.Watchers) != 1 || got.Watchers[0].DisplayName != "Ada" {
		t.Errorf("got=%+v err=%v", got, err)
	}
	if err := api.AddWatcher(c, "PROJ-1", "a1"); err != nil || method != http.MethodPost || body != `"a1"` {
		t.Errorf("add: method=%s body=%s err=%v", method, body, err)
	}
	if err := api.RemoveWatcher(c, "PROJ-1", "557058:x y"); err != nil || method != http.MethodDelete || query != "accountId=557058%3Ax+y" {
		t.Errorf("remove: method=%s query=%s err=%v", method, query, err)
	}
}

func TestWatchers_RejectBadKey(t *testing.T) {
	c := conn("https://unused.invalid")
	if _, err := api.FetchWatchers(c, "../x"); err == nil {
		t.Error("fetch: expected error")
	}
	if err := api.AddWatcher(c, "../x", "a1"); err == nil {
		t.Error("add: expected error")
	}
	if err := api.RemoveWatcher(c, "../x", "a1"); err == nil {
		t.Error("remove: expected error")
	}
}

func TestFetchEditMeta(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/PROJ-1/editmeta" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{
			"issuetype": map[string]any{"name": "Issue Type", "allowedValues": []map[string]any{{"id": "1", "name": "Bug"}}},
		}})
	}))
	defer srv.Close()
	meta, err := api.FetchEditMeta(conn(srv.URL), "PROJ-1")
	if err != nil || meta["issuetype"].AllowedValues[0]["name"] != "Bug" {
		t.Errorf("meta=%v err=%v", meta, err)
	}
	if _, err := api.FetchEditMeta(conn(srv.URL), "../x"); err == nil {
		t.Error("expected bad key error")
	}
}
