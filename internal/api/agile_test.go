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

func TestFetchSprints_PaginatesUntilLast(t *testing.T) {
	var starts []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("startAt")
		starts = append(starts, start)
		if r.URL.Query().Get("state") != "closed" {
			t.Errorf("state = %q", r.URL.Query().Get("state"))
		}
		page := map[string]any{"values": []map[string]any{{"id": 1, "name": "S1", "state": "closed"}}, "isLast": false}
		if start != "0" {
			page = map[string]any{"values": []map[string]any{{"id": 2, "name": "S2", "state": "closed", "completeDate": "2026-01-02"}}, "isLast": true}
		}
		json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	sprints, err := api.FetchSprints(conn(srv.URL), 1, "closed")
	if err != nil {
		t.Fatal(err)
	}
	if len(sprints) != 2 || sprints[1].CompleteDate != "2026-01-02" || strings.Join(starts, ",") != "0,1" {
		t.Errorf("sprints=%+v starts=%v", sprints, starts)
	}
}

func TestFetchBoards_EscapesProject(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("projectKeyOrId") != "A&b=c" {
			t.Errorf("raw query = %s", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode(map[string]any{"values": []any{}})
	}))
	defer srv.Close()

	if _, err := api.FetchBoards(conn(srv.URL), "A&b=c"); err != nil {
		t.Fatal(err)
	}
}

func TestFetchSprintIssues_AsksForListFields(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !strings.Contains(q.Get("fields"), "summary") || q.Get("jql") != "assignee = currentUser()" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode(map[string]any{"issues": []any{}})
	}))
	defer srv.Close()

	if _, err := api.FetchSprintIssues(conn(srv.URL), 5, "assignee = currentUser()"); err != nil {
		t.Fatal(err)
	}
}

func TestAddIssuesToSprint_RejectsBadKey(t *testing.T) {
	if err := api.AddIssuesToSprint(conn("https://example.invalid"), 5, []string{"OK-1", "../x"}); err == nil {
		t.Error("expected invalid key error")
	}
}
