package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

func TestCreateRequest_CarriesConfigDefaults(t *testing.T) {
	withConfig(t, config.Config{DefaultPriority: "High", DefaultLabels: []string{"cfg"}})
	got, err := createRequest(newCallToolRequest(map[string]any{"project": "PROJ", "summary": "S"}))
	if err != nil || got.Defaults.Priority != "High" || len(got.Defaults.Labels) != 1 {
		t.Errorf("req=%+v err=%v", got, err)
	}
}

func TestHandleCreateTicketsBatch_AppliesDefaults(t *testing.T) {
	withConfig(t, config.Config{DefaultPriority: "High"})
	var fields map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		fields, _ = body["fields"].(map[string]any)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1"})
	}))
	t.Cleanup(srv.Close)

	handleCreateTicketsBatch(api.JiraConnection{BaseURL: srv.URL})(context.Background(),
		newCallToolRequest(map[string]any{"tickets": `[{"project":"PROJ","summary":"One"}]`}))
	priority, _ := fields["priority"].(map[string]any)
	if priority["name"] != "High" {
		t.Errorf("created fields = %v", fields)
	}
}
