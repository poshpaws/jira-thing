package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func TestFetchEpics_ReturnsEpicsInProject(t *testing.T) {
	var capturedJQL string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		capturedJQL, _ = body["jql"].(string)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{map[string]any{"key": "PROJ-1"}}, "total": 1,
		})
	}))
	defer srv.Close()

	result, err := api.FetchEpics(conn(srv.URL), "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("got total %d, want 1", result.Total)
	}
	if !strings.Contains(capturedJQL, `project = "PROJ"`) || !strings.Contains(capturedJQL, "issuetype = Epic") {
		t.Errorf("unexpected JQL: %s", capturedJQL)
	}
}

func TestFetchEpics_500(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := api.FetchEpics(conn(srv.URL), "PROJ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListEpicIssues_ParentFieldWorks(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{map[string]any{"key": "PROJ-2"}}, "total": 1,
		})
	}))
	defer srv.Close()

	result, err := api.ListEpicIssues(conn(srv.URL), "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("got total %d, want 1", result.Total)
	}
}

func TestListEpicIssues_FallsBackToEpicLinkField(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/search/jql"):
			calls++
			if calls == 1 {
				// "parent = ..." query returns nothing (team-managed field doesn't apply).
				json.NewEncoder(w).Encode(map[string]any{"issues": []any{}, "total": 0})
				return
			}
			// Second call uses the Epic Link custom field JQL.
			json.NewEncoder(w).Encode(map[string]any{
				"issues": []any{map[string]any{"key": "PROJ-2"}}, "total": 1,
			})
		case strings.HasSuffix(r.URL.Path, "/field"):
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "customfield_10014", "name": "Epic Link", "custom": true},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	result, err := api.ListEpicIssues(conn(srv.URL), "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("got total %d, want 1 (expected Epic Link fallback)", result.Total)
	}
}

func TestAddIssuesToEpic_ParentFieldSucceeds(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/field"):
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	err := api.AddIssuesToEpic(conn(srv.URL), "EPIC-1", []string{"PROJ-1", "PROJ-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddIssuesToEpic_ReportsFailures(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			http.Error(w, `{"errorMessages":["cannot set parent"]}`, http.StatusBadRequest)
		case strings.HasSuffix(r.URL.Path, "/field"):
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	err := api.AddIssuesToEpic(conn(srv.URL), "EPIC-1", []string{"PROJ-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "PROJ-1") {
		t.Errorf("expected failure to mention PROJ-1, got: %v", err)
	}
}

func TestBuildEpicFields_SetsEpicNameWhenFieldExists(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "customfield_10011", "name": "Epic Name", "custom": true},
		})
	}))
	defer srv.Close()

	fields := api.BuildEpicFields(conn(srv.URL), "PROJ", "My Epic Name", "My Epic Summary")
	if fields["customfield_10011"] != "My Epic Name" {
		t.Errorf("expected Epic Name field to be set, got: %+v", fields)
	}
	if fields["summary"] != "My Epic Summary" {
		t.Errorf("expected summary to be set, got: %+v", fields)
	}
}

func TestBuildEpicFields_OmitsEpicNameWhenFieldMissing(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	fields := api.BuildEpicFields(conn(srv.URL), "PROJ", "My Epic Name", "My Epic Summary")
	for k := range fields {
		if strings.HasPrefix(k, "customfield_") {
			t.Errorf("did not expect a custom field to be set, got: %+v", fields)
		}
	}
}
