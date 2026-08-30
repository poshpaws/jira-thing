package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

func TestFetchIssueLinkTypes_ReturnsList(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/issueLinkType") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issueLinkTypes": []map[string]any{
				{"id": "10000", "name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
			},
		})
	}))
	defer srv.Close()

	types, err := api.FetchIssueLinkTypes(conn(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 || types[0].Name != "Blocks" || types[0].Outward != "blocks" {
		t.Errorf("unexpected result: %+v", types)
	}
}

func TestFetchIssueLinkTypes_500(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := api.FetchIssueLinkTypes(conn(srv.URL))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLinkIssues_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		linkType, _ := body["type"].(map[string]any)
		if linkType["name"] != "Blocks" {
			t.Errorf("got link type %v, want Blocks", linkType["name"])
		}
		outward, _ := body["outwardIssue"].(map[string]any)
		if outward["key"] != "PROJ-1" {
			t.Errorf("got outward key %v, want PROJ-1", outward["key"])
		}
		inward, _ := body["inwardIssue"].(map[string]any)
		if inward["key"] != "PROJ-2" {
			t.Errorf("got inward key %v, want PROJ-2", inward["key"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	err := api.LinkIssues(conn(srv.URL), "PROJ-1", "PROJ-2", "Blocks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnlinkIssues_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/PROJ-1"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"fields": map[string]any{
					"issuelinks": []map[string]any{
						{"id": "10001", "outwardIssue": map[string]any{"key": "PROJ-2"}},
					},
				},
			})
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/issueLink/10001"):
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	if err := api.UnlinkIssues(conn(srv.URL), "PROJ-1", "PROJ-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnlinkIssues_NoLinkFound(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"fields": map[string]any{"issuelinks": []any{}}})
	}))
	defer srv.Close()

	err := api.UnlinkIssues(conn(srv.URL), "PROJ-1", "PROJ-2")
	if err == nil {
		t.Fatal("expected error when no link exists, got nil")
	}
}

func TestAddRemoteLink_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/PROJ-1/remotelink") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		obj, _ := body["object"].(map[string]any)
		if obj["url"] != "https://example.com" {
			t.Errorf("got url %v", obj["url"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	err := api.AddRemoteLink(conn(srv.URL), "PROJ-1", "https://example.com", "Example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddRemoteLink_400(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["bad url"]}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	err := api.AddRemoteLink(conn(srv.URL), "PROJ-1", "not-a-url", "Example")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLinkIssues_400(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errorMessages":["invalid link type"]}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	err := api.LinkIssues(conn(srv.URL), "PROJ-1", "PROJ-2", "BadType")
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}
