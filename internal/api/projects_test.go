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
