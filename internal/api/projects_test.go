package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func TestFetchProjects_ReturnsList(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/project/search") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{{"id": "10000", "key": "PROJ", "name": "Project"}},
		})
	}))
	defer srv.Close()

	projects, err := api.FetchProjects(conn(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].Key != "PROJ" {
		t.Errorf("unexpected result: %+v", projects)
	}
}

func TestFetchProjects_500(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := api.FetchProjects(conn(srv.URL))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchProjectVersions_ReturnsList(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/PROJ/version") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{{"id": "1", "name": "v1.0", "released": true, "archived": false}},
		})
	}))
	defer srv.Close()

	versions, err := api.FetchProjectVersions(conn(srv.URL), "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 || versions[0].Name != "v1.0" || !versions[0].Released {
		t.Errorf("unexpected result: %+v", versions)
	}
}

func TestFetchProjectVersions_404(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := api.FetchProjectVersions(conn(srv.URL), "BAD")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchProjects_PaginatesUntilLast(t *testing.T) {
	var starts []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		starts = append(starts, r.URL.Query().Get("startAt"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{{"key": "P" + r.URL.Query().Get("startAt")}},
			"isLast": len(starts) == 2,
		})
	}))
	defer srv.Close()
	projects, err := api.FetchProjects(conn(srv.URL))
	if err != nil || len(projects) != 2 || strings.Join(starts, ",") != "0,1" {
		t.Errorf("projects=%+v starts=%v err=%v", projects, starts, err)
	}
}

func TestFetchProjectVersions_RejectsBadKey(t *testing.T) {
	for _, key := range []string{"", "../myself", "PROJ?x=1", "PR OJ"} {
		if _, err := api.FetchProjectVersions(conn("https://unused.invalid"), key); err == nil {
			t.Errorf("key %q: expected an error", key)
		}
	}
}

func TestFetchProjectVersions_OrdersByReleaseDate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("orderBy") != "-releaseDate" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"isLast": true, "values": []map[string]any{
			{"name": "v2", "releaseDate": "2026-12-01", "overdue": true},
		}})
	}))
	defer srv.Close()
	versions, err := api.FetchProjectVersions(conn(srv.URL), "PROJ")
	if err != nil || len(versions) != 1 || versions[0].ReleaseDate != "2026-12-01" || !versions[0].Overdue {
		t.Errorf("versions=%+v err=%v", versions, err)
	}
}

func TestFetchRecentComments_NewestNOldestFirst(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("orderBy") != "-created" || r.URL.Query().Get("maxResults") != "2" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"total": 5, "comments": []map[string]any{
			{"created": "2026-11-16"}, {"created": "2026-11-15"},
		}})
	}))
	defer srv.Close()
	res, err := api.FetchRecentComments(conn(srv.URL), "PROJ-1", 2)
	if err != nil || res.Total != 5 || len(res.Comments) != 2 || res.Comments[0].Created != "2026-11-15" {
		t.Errorf("res=%+v err=%v", res, err)
	}
	if _, err := api.FetchRecentComments(conn(srv.URL), "../x", 2); err == nil {
		t.Error("expected bad key error")
	}
}
