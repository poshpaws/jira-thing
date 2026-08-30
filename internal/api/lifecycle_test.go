package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func TestDeleteIssue_Cascade(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected method %s", r.Method)
		}
		if r.URL.Query().Get("deleteSubtasks") != "true" {
			t.Errorf("expected deleteSubtasks=true, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := api.DeleteIssue(conn(srv.URL), "PROJ-1", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteIssue_NoCascade(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("deleteSubtasks") != "false" {
			t.Errorf("expected deleteSubtasks=false, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := api.DeleteIssue(conn(srv.URL), "PROJ-1", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteIssue_409HasSubtasks(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["subtasks exist"]}`, http.StatusConflict)
	}))
	defer srv.Close()

	err := api.DeleteIssue(conn(srv.URL), "PROJ-1", false)
	if err == nil {
		t.Fatal("expected error for 409, got nil")
	}
}

func TestAddWorklog_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/PROJ-1/worklog") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		if body["timeSpent"] != "2h" {
			t.Errorf("got timeSpent %v, want 2h", body["timeSpent"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "1"})
	}))
	defer srv.Close()

	if err := api.AddWorklog(conn(srv.URL), "PROJ-1", "2h", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddWorklog_400(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["bad duration"]}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	err := api.AddWorklog(conn(srv.URL), "PROJ-1", "bad", nil)
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}
