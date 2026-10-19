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
