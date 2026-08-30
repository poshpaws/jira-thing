package api_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func init() {
	// Allow test TLS servers with self-signed certs.
	api.SetHTTPClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- test only
		},
	})
}

func conn(baseURL string) api.JiraConnection {
	return api.JiraConnection{BaseURL: baseURL, Email: "a@b.com", APIToken: "tok"}
}

func TestFetchIssue_ReturnsJSON(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := api.FetchIssue(conn(srv.URL), "BAD-1")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestCreateIssue_ReturnsKey(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errors":{"summary":"required"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := api.CreateIssue(conn(srv.URL), map[string]any{})
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if !strings.Contains(err.Error(), `"summary"`) {
		t.Errorf("error should include response body, got: %v", err)
	}
}

func TestSearchIssues_ReturnsIssues(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	q := api.SearchQuery{JQL: "assignee=currentUser()", MaxResults: 10}
	_, err := api.SearchIssues(conn(url), q)
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestAddAttachment_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/attachments") {
			t.Errorf("expected /attachments path, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			t.Errorf("expected X-Atlassian-Token: no-check header")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("reading form file: %v", err)
		}
		defer file.Close()
		if header.Filename != "note.txt" {
			t.Errorf("filename = %q, want note.txt", header.Filename)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]any{map[string]any{"id": "999", "filename": "note.txt"}})
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	result, err := api.AddAttachment(conn(srv.URL), "PROJ-1", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0]["filename"] != "note.txt" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestAddAttachment_MissingFile(t *testing.T) {
	_, err := api.AddAttachment(conn("http://unused"), "PROJ-1", "/no/such/file.txt")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestAddAttachment_404(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	_, err := api.AddAttachment(conn(srv.URL), "BAD-1", path)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestAddAttachment_InvalidURL(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_, err := api.AddAttachment(api.JiraConnection{BaseURL: "http://\x00invalid"}, "PROJ-1", path)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestFetchMyself_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/myself") {
			t.Errorf("expected /myself path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"accountId":    "abc123",
			"displayName":  "Test User",
			"emailAddress": "test@example.com",
		})
	}))
	defer srv.Close()

	result, err := api.FetchMyself(conn(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["accountId"] != "abc123" {
		t.Errorf("accountId = %v, want abc123", result["accountId"])
	}
	if result["displayName"] != "Test User" {
		t.Errorf("displayName = %v, want Test User", result["displayName"])
	}
}

func TestFetchMyself_401(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := api.FetchMyself(conn(srv.URL))
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestFetchMyself_InvalidURL(t *testing.T) {
	_, err := api.FetchMyself(api.JiraConnection{BaseURL: "http://\x00invalid"})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestFetchTransitions_ReturnsList(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/PROJ-1/transitions") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"transitions": []map[string]any{
				{"id": "11", "name": "In Progress", "to": map[string]any{"id": "3", "name": "In Progress"}},
				{"id": "21", "name": "Done", "to": map[string]any{"id": "5", "name": "Done"}},
			},
		})
	}))
	defer srv.Close()

	transitions, err := api.FetchTransitions(conn(srv.URL), "PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transitions) != 2 {
		t.Fatalf("got %d transitions, want 2", len(transitions))
	}
	if transitions[0].ID != "11" || transitions[0].Name != "In Progress" || transitions[0].To.Name != "In Progress" {
		t.Errorf("unexpected transition: %+v", transitions[0])
	}
}

func TestFetchTransitions_404(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := api.FetchTransitions(conn(srv.URL), "BAD-1")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestUpdateIssue_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/PROJ-1") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		fields, _ := body["fields"].(map[string]any)
		if fields["summary"] != "New summary" {
			t.Errorf("got summary %v, want New summary", fields["summary"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := api.UpdateIssue(conn(srv.URL), "PROJ-1", map[string]any{"summary": "New summary"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateIssue_400(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["invalid field"]}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	err := api.UpdateIssue(conn(srv.URL), "PROJ-1", map[string]any{"summary": "x"})
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}

func TestTransitionIssue_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		transition, _ := body["transition"].(map[string]any)
		if transition["id"] != "21" {
			t.Errorf("got transition id %v, want 21", transition["id"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := api.TransitionIssue(conn(srv.URL), "PROJ-1", "21"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransitionIssue_400(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["invalid transition"]}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	err := api.TransitionIssue(conn(srv.URL), "PROJ-1", "99")
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}
