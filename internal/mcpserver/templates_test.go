package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
)

// batchServer creates tickets PROJ-1, PROJ-2, ... and counts the creates.
func batchServer(t *testing.T, creates *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]any{"key": "SRC-1", "fields": map[string]any{
				"project": map[string]any{"key": "PROJ"}, "summary": "S",
				"assignee": map[string]any{"accountId": "a1", "displayName": "Al"},
			}})
			return
		}
		*creates++
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-" + string(rune('0'+*creates))})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	text, _ := result.Content[0].(mcp.TextContent)
	return text.Text
}

func TestHandleCreateTicketsBatch_ReportsEachEntry(t *testing.T) {
	creates := 0
	srv := batchServer(t, &creates)
	tickets := `[{"project":"PROJ","summary":"One","description":"**md**"},{"project":"PROJ"},{"project":"PROJ","summary":"Three"}]`
	result, _ := handleCreateTicketsBatch(api.JiraConnection{BaseURL: srv.URL})(context.Background(),
		newCallToolRequest(map[string]any{"tickets": tickets}))
	text := resultText(t, result)
	if creates != 2 || !strings.Contains(text, "2 of 3 created") || !strings.Contains(text, "summary is required") {
		t.Errorf("creates=%d text=%s", creates, text)
	}
}

func TestHandleCreateTicketsBatch_DryRunAndBadJSON(t *testing.T) {
	creates := 0
	srv := batchServer(t, &creates)
	conn := api.JiraConnection{BaseURL: srv.URL}
	result, _ := handleCreateTicketsBatch(conn)(context.Background(),
		newCallToolRequest(map[string]any{"tickets": `[{"project":"PROJ","summary":"A"}]`, "dry_run": true}))
	if creates != 0 || !strings.Contains(resultText(t, result), "Would create") {
		t.Errorf("creates=%d text=%s", creates, resultText(t, result))
	}
	bad, _ := handleCreateTicketsBatch(conn)(context.Background(), newCallToolRequest(map[string]any{"tickets": "{}"}))
	if !bad.IsError {
		t.Errorf("expected an error for a non-array, got %+v", bad)
	}
}

func TestHandleFetchTemplate_DefaultsToCreatorMarker(t *testing.T) {
	creates := 0
	srv := batchServer(t, &creates)
	conn := api.JiraConnection{BaseURL: srv.URL}
	result, _ := handleFetchTemplate(conn)(context.Background(), newCallToolRequest(map[string]any{"ticket_key": "SRC-1"}))
	if text := resultText(t, result); !strings.Contains(text, "__SELF__") || !strings.Contains(text, `"PROJ"`) {
		t.Errorf("text=%s", text)
	}
	kept, _ := handleFetchTemplate(conn)(context.Background(),
		newCallToolRequest(map[string]any{"ticket_key": "SRC-1", "keep_assignee": true}))
	if !strings.Contains(resultText(t, kept), `"a1"`) {
		t.Errorf("expected original assignee kept, got %s", resultText(t, kept))
	}
}

func TestFormatDiagnosis(t *testing.T) {
	failed := formatDiagnosis(ops.Diagnosis{URL: "u", APIErr: errors.New("HTTP 401"), Hint: "regenerate"})
	if !strings.Contains(failed, "FAIL API connection") || !strings.Contains(failed, "Hint: regenerate") {
		t.Errorf("failed=%s", failed)
	}
	passed := formatDiagnosis(ops.Diagnosis{User: ops.User{DisplayName: "Me", AccountID: "a1"}})
	if !strings.Contains(passed, "All checks passed") || !strings.Contains(passed, "a1") {
		t.Errorf("passed=%s", passed)
	}
}
