package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchConfluencePage_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/content" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("spaceKey") != "ENG" {
			t.Errorf("spaceKey = %q, want ENG", r.URL.Query().Get("spaceKey"))
		}
		if r.URL.Query().Get("title") != "Toil Tracker" {
			t.Errorf("title = %q, want Toil Tracker", r.URL.Query().Get("title"))
		}
		if !strings.Contains(r.URL.Query().Get("expand"), "version") {
			t.Errorf("expand missing version: %s", r.URL.Query().Get("expand"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"id":    "12345",
					"title": "Toil Tracker",
					"version": map[string]any{
						"number": float64(5),
					},
				},
			},
			"size": 1,
		})
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	page, err := FetchConfluencePage(conn, "ENG", "Toil Tracker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.ID != "12345" {
		t.Errorf("ID = %q, want 12345", page.ID)
	}
	if page.Version != 5 {
		t.Errorf("Version = %d, want 5", page.Version)
	}
	if page.Title != "Toil Tracker" {
		t.Errorf("Title = %q, want Toil Tracker", page.Title)
	}
}

func TestFetchConfluencePage_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "size": 0})
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	_, err := FetchConfluencePage(conn, "ENG", "Missing Page")
	if err == nil {
		t.Fatal("expected error for missing page")
	}
	if !strings.Contains(err.Error(), "Missing Page") {
		t.Errorf("error should mention page title: %v", err)
	}
}

func TestFetchConfluencePage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	_, err := FetchConfluencePage(conn, "ENG", "Toil Tracker")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestUpdateConfluencePage_Success(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/wiki/rest/api/content/12345" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "12345"})
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	if err := UpdateConfluencePage(conn, "12345", 5, "Toil Tracker", "<p>content</p>"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ver, _ := capturedBody["version"].(map[string]any)
	num, _ := ver["number"].(float64)
	if int(num) != 6 {
		t.Errorf("version.number = %v, want 6", num)
	}
	body, _ := capturedBody["body"].(map[string]any)
	storage, _ := body["storage"].(map[string]any)
	if storage["value"] != "<p>content</p>" {
		t.Errorf("body.storage.value = %v, want <p>content</p>", storage["value"])
	}
	if storage["representation"] != "storage" {
		t.Errorf("body.storage.representation = %v, want storage", storage["representation"])
	}
}

func TestUpdateConfluencePage_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	if err := UpdateConfluencePage(conn, "12345", 5, "Toil Tracker", "<p>x</p>"); err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestListChildPages_ReturnsPagesWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/content/99/child/page" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.Query().Get("expand"), "body.storage") {
			t.Errorf("missing body.storage expand: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"id": "101", "title": "CRSS-1",
					"version": map[string]any{"number": float64(2)},
					"body":    map[string]any{"storage": map[string]any{"value": "<p>existing</p>"}},
				},
			},
			"size": 1,
		})
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	pages, err := ListChildPages(conn, "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("len = %d, want 1", len(pages))
	}
	if pages[0].ID != "101" || pages[0].Title != "CRSS-1" || pages[0].Version != 2 {
		t.Errorf("unexpected page: %+v", pages[0])
	}
	if pages[0].Body != "<p>existing</p>" {
		t.Errorf("Body = %q, want <p>existing</p>", pages[0].Body)
	}
}

func TestListChildPages_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "size": 0})
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	pages, err := ListChildPages(conn, "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("expected empty slice, got %d pages", len(pages))
	}
}

func TestCreateConfluencePage_Success(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/wiki/rest/api/content" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "200", "title": "CRSS-1",
			"version": map[string]any{"number": float64(1)},
		})
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	page, err := CreateConfluencePage(conn, "ENG", "CRSS-1", "99", "<p>body</p>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.ID != "200" {
		t.Errorf("ID = %q, want 200", page.ID)
	}
	// Verify ancestors and space sent correctly
	ancestors, _ := capturedBody["ancestors"].([]any)
	if len(ancestors) != 1 {
		t.Fatalf("expected 1 ancestor, got %v", capturedBody["ancestors"])
	}
	anc, _ := ancestors[0].(map[string]any)
	if anc["id"] != "99" {
		t.Errorf("ancestor id = %v, want 99", anc["id"])
	}
	space, _ := capturedBody["space"].(map[string]any)
	if space["key"] != "ENG" {
		t.Errorf("space.key = %v, want ENG", space["key"])
	}
}

func TestCreateConfluencePage_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	if _, err := CreateConfluencePage(conn, "ENG", "CRSS-1", "99", "<p>body</p>"); err == nil {
		t.Fatal("expected error for 400")
	}
}
