package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func TestFetchBoards_FiltersByProject(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/board") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("projectKeyOrId") != "PROJ" {
			t.Errorf("expected projectKeyOrId=PROJ, got %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{{"id": 1, "name": "Board 1", "type": "scrum"}},
		})
	}))
	defer srv.Close()

	boards, err := api.FetchBoards(conn(srv.URL), "PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(boards) != 1 || boards[0].Name != "Board 1" {
		t.Errorf("unexpected result: %+v", boards)
	}
}

func TestFetchSprints_ReturnsList(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/board/1/sprint") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{{"id": 5, "name": "Sprint 5", "state": "active"}},
		})
	}))
	defer srv.Close()

	sprints, err := api.FetchSprints(conn(srv.URL), 1, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sprints) != 1 || sprints[0].State != "active" {
		t.Errorf("unexpected result: %+v", sprints)
	}
}

func TestFetchSprintIssues_ReturnsIssues(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sprint/5/issue") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{map[string]any{"key": "PROJ-1"}}, "total": 1,
		})
	}))
	defer srv.Close()

	result, err := api.FetchSprintIssues(conn(srv.URL), 5, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("got total %d, want 1", result.Total)
	}
}

func TestAddIssuesToSprint_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		issues, _ := body["issues"].([]any)
		if len(issues) != 2 {
			t.Errorf("got %d issues, want 2", len(issues))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := api.AddIssuesToSprint(conn(srv.URL), 5, []string{"PROJ-1", "PROJ-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddIssuesToSprint_400(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["invalid issue"]}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	err := api.AddIssuesToSprint(conn(srv.URL), 5, []string{"BAD-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
