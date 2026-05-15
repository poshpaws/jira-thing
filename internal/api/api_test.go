package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func conn(baseURL string) api.JiraConnection {
	return api.JiraConnection{BaseURL: baseURL, Email: "a@b.com", APIToken: "tok"}
}

func TestFetchIssue_ReturnsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1"})
	}))
	defer srv.Close()

	result, err := api.FetchIssue(conn(srv.URL), "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "PROJ-1" {
		t.Errorf("got key %v, want PROJ-1", result["key"])
	}
}

func TestFetchIssue_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := api.FetchIssue(conn(srv.URL), "BAD-1")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestCreateIssue_ReturnsKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-2"})
	}))
	defer srv.Close()

	result, err := api.CreateIssue(conn(srv.URL), map[string]any{"summary": "New ticket"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "PROJ-2" {
		t.Errorf("got key %v, want PROJ-2", result["key"])
	}
}

func TestCreateIssue_400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errors":{"summary":"required"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := api.CreateIssue(conn(srv.URL), map[string]any{})
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}

func TestCreateIssue_RetriesOnUnsupportedFields(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var payload struct {
			Fields map[string]any `json:"fields"`
		}
		json.NewDecoder(r.Body).Decode(&payload)

		if attempts == 1 {
			http.Error(w, `{"errors":{"customfield_10038":"Field does not support update 'customfield_10038'","customfield_10020":"The Sprint (id) must be a number","rankBeforeIssue":"expected Object"}}`, http.StatusBadRequest)
			return
		}
		// Verify all three bad fields were stripped
		for _, field := range []string{"customfield_10038", "customfield_10020", "rankBeforeIssue"} {
			if _, present := payload.Fields[field]; present {
				t.Errorf("%s should have been stripped on retry", field)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-3"})
	}))
	defer srv.Close()

	fields := map[string]any{
		"summary":           "Test",
		"customfield_10038": "bad",
		"customfield_10020": map[string]any{"id": "sprint-abc"},
		"rankBeforeIssue":   map[string]any{},
	}
	result, err := api.CreateIssue(conn(srv.URL), fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "PROJ-3" {
		t.Errorf("got key %v, want PROJ-3", result["key"])
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestCreateIssue_NoRetryOnCoreFieldErrors(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, `{"errors":{"summary":"Field 'summary' cannot be empty"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := api.CreateIssue(conn(srv.URL), map[string]any{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on core field error), got %d", attempts)
	}
}

func TestSearchIssues_ReturnsIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		if body["jql"] == "" {
			t.Error("expected jql in request body")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{
				map[string]any{"key": "PROJ-1", "fields": map[string]any{"summary": "Task one"}},
				map[string]any{"key": "PROJ-2", "fields": map[string]any{"summary": "Task two"}},
			},
			"total":      2,
			"maxResults": 100,
		})
	}))
	defer srv.Close()

	q := api.SearchQuery{JQL: "assignee=currentUser()", Fields: []string{"summary", "status"}, MaxResults: 100}
	result, err := api.SearchIssues(conn(srv.URL), q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if len(result.Issues) != 2 {
		t.Fatalf("issues len = %d, want 2", len(result.Issues))
	}
	if result.Issues[0]["key"] != "PROJ-1" {
		t.Errorf("first issue key = %v, want PROJ-1", result.Issues[0]["key"])
	}
}

func TestSearchIssues_400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["bad jql"]}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	q := api.SearchQuery{JQL: "INVALID JQL %%%", MaxResults: 50}
	_, err := api.SearchIssues(conn(srv.URL), q)
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}

func TestAddComment_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/comment") {
			t.Errorf("expected /comment path, got %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		if body["body"] == nil {
			t.Error("expected body in payload")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": "12345"})
	}))
	defer srv.Close()

	body := map[string]any{"type": "doc", "version": 1, "content": []any{}}
	err := api.AddComment(conn(srv.URL), "PROJ-1", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddComment_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	err := api.AddComment(conn(srv.URL), "BAD-1", map[string]any{})
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestAddComment_InvalidURL(t *testing.T) {
	err := api.AddComment(api.JiraConnection{BaseURL: "http://\x00invalid"}, "PROJ-1", map[string]any{})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestFetchIssue_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	_, err := api.FetchIssue(conn(url), "PROJ-1")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestFetchIssue_InvalidURL(t *testing.T) {
	_, err := api.FetchIssue(api.JiraConnection{BaseURL: "http://\x00invalid"}, "PROJ-1")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestCreateIssue_InvalidURL(t *testing.T) {
	_, err := api.CreateIssue(api.JiraConnection{BaseURL: "http://\x00invalid"}, map[string]any{"summary": "x"})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestSearchIssues_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	q := api.SearchQuery{JQL: "assignee=currentUser()", MaxResults: 10}
	_, err := api.SearchIssues(conn(url), q)
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}
