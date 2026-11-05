package mcpserver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
)

func init() {
	// Allow test TLS servers with self-signed certs.
	api.SetHTTPClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- test only
		},
	})
}

func newCallToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func TestHandleCreateTicket_ComponentsAndAssignee(t *testing.T) {
	var capturedFields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/myself"):
			json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-42"})
		case strings.HasSuffix(r.URL.Path, "/issue"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			capturedFields, _ = body["fields"].(map[string]any)
			json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	conn := api.JiraConnection{BaseURL: srv.URL}
	handler := handleCreateTicket(conn)

	req := newCallToolRequest(map[string]any{
		"project":    "PROJ",
		"summary":    "Fix the thing",
		"components": "Backend, API",
		"assignee":   "me",
	})
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error result: %+v", result.Content)
	}

	components, _ := capturedFields["components"].([]any)
	if len(components) != 2 {
		t.Fatalf("components = %v, want 2 entries", capturedFields["components"])
	}
	if c0, ok := components[0].(map[string]any); !ok || c0["name"] != "Backend" {
		t.Errorf("components[0] = %v, want Backend", components[0])
	}

	assignee, ok := capturedFields["assignee"].(map[string]any)
	if !ok || assignee["accountId"] != "acc-42" {
		t.Errorf("assignee = %v, want resolved acc-42", capturedFields["assignee"])
	}
}

func TestHandleCreateTicket_MissingSummaryIsToolError(t *testing.T) {
	conn := api.JiraConnection{BaseURL: "https://example.invalid"}
	handler := handleCreateTicket(conn)

	result, err := handler(context.Background(), newCallToolRequest(map[string]any{"project": "PROJ"}))
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected a tool error result for missing summary")
	}
}

func TestFormatIssue_ListsLinkedTickets(t *testing.T) {
	issue := map[string]any{
		"key": "PROJ-1",
		"fields": map[string]any{
			"summary": "Fix the thing",
			"issuelinks": []any{
				map[string]any{
					"type": map[string]any{"inward": "is blocked by", "outward": "blocks"},
					"outwardIssue": map[string]any{
						"key":    "PROJ-2",
						"fields": map[string]any{"summary": "Downstream", "status": map[string]any{"name": "To Do"}},
					},
				},
			},
		},
	}
	got := formatIssue(issue)
	want := "## Linked Tickets\n\n- blocks PROJ-2: Downstream (To Do)\n"
	if !strings.Contains(got, want) {
		t.Errorf("missing linked tickets section:\n%s", got)
	}
}

func TestFormatIssue_NoLinksOmitsSection(t *testing.T) {
	got := formatIssue(map[string]any{"key": "PROJ-1", "fields": map[string]any{"summary": "x"}})
	if strings.Contains(got, "Linked Tickets") {
		t.Errorf("unexpected linked tickets section:\n%s", got)
	}
}

// dedupeServer fakes search and create, counting creates. searchStatus 200
// answers with one open ticket summarised "Fix the thing".
func dedupeServer(t *testing.T, searchStatus int, creates *int) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, api.SearchEndpoint) {
			w.WriteHeader(searchStatus)
			json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{
				{"key": "PROJ-7", "fields": map[string]any{"summary": "Fix the thing"}},
			}})
			return
		}
		*creates++
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-8"})
	}))
}

func callCreateTicket(t *testing.T, srvURL string, dedupe bool) *mcp.CallToolResult {
	t.Helper()
	result, err := handleCreateTicket(api.JiraConnection{BaseURL: srvURL})(context.Background(), newCallToolRequest(map[string]any{
		"project": "PROJ", "summary": "Fix the thing", "dedupe_by_summary": dedupe,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

func TestHandleCreateTicket_DedupeSkipsDuplicate(t *testing.T) {
	creates := 0
	srv := dedupeServer(t, http.StatusOK, &creates)
	defer srv.Close()

	result := callCreateTicket(t, srv.URL, true)
	text, _ := result.Content[0].(mcp.TextContent)
	if result.IsError || creates != 0 || !strings.Contains(text.Text, "PROJ-7") {
		t.Errorf("creates=%d result=%+v", creates, result)
	}
}

func TestHandleCreateTicket_DedupeSearchFailureCreatesNothing(t *testing.T) {
	creates := 0
	srv := dedupeServer(t, http.StatusInternalServerError, &creates)
	defer srv.Close()

	if result := callCreateTicket(t, srv.URL, true); !result.IsError || creates != 0 {
		t.Errorf("creates=%d result=%+v", creates, result)
	}
}

func TestHandleCreateTicket_NoDedupeCreatesDespiteDuplicate(t *testing.T) {
	creates := 0
	srv := dedupeServer(t, http.StatusOK, &creates)
	defer srv.Close()

	if result := callCreateTicket(t, srv.URL, false); result.IsError || creates != 1 {
		t.Errorf("creates=%d result=%+v", creates, result)
	}
}
