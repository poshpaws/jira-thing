package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

func TestCustomFields(t *testing.T) {
	req := newCallToolRequest(map[string]any{"custom_fields": map[string]any{
		"Story Points": 2.5, "Team": "Platform", "Tags": []any{"a", "b"}, "Clear": nil, "Flag": true,
	}})
	got, err := customFields(req)
	if err != nil {
		t.Fatal(err)
	}
	want := "Clear=|Flag=true|Story Points=2.5|Tags=a,b|Team=Platform"
	parts := make([]string, len(got))
	for i, v := range got {
		parts[i] = v.Field + "=" + v.Value
	}
	if strings.Join(parts, "|") != want {
		t.Errorf("got %s", strings.Join(parts, "|"))
	}
	if _, err := customFields(newCallToolRequest(map[string]any{"custom_fields": "Story Points=3"})); err == nil {
		t.Error("a string should be rejected")
	}
	if got, err := customFields(newCallToolRequest(map[string]any{})); got != nil || err != nil {
		t.Errorf("absent: got=%v err=%v", got, err)
	}
}

func TestUpdateAndCreate_CustomFields(t *testing.T) {
	withConfig(t, config.Config{})
	var posted map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/field"):
			json.NewEncoder(w).Encode([]map[string]any{{"id": "customfield_7", "name": "Team Size", "schema": map[string]any{"type": "number"}}})
		case r.Method == http.MethodPost:
			json.NewDecoder(r.Body).Decode(&posted)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"key": "P-9"})
		}
	}))
	t.Cleanup(srv.Close)
	conn := api.JiraConnection{BaseURL: srv.URL}
	text := callAgile(t, handleUpdateTicket, conn, map[string]any{"ticket_key": "K-1", "custom_fields": map[string]any{"team size": 4.0}, "dry_run": true})
	if !strings.Contains(text, `"customfield_7": 4`) {
		t.Errorf("update text = %s", text)
	}
	text = callAgile(t, handleCreateTicket, conn, map[string]any{"project": "P", "summary": "S", "custom_fields": map[string]any{"Team Size": 3.0}})
	fields, _ := posted["fields"].(map[string]any)
	if !strings.Contains(text, "P-9") || fields["customfield_7"] != 3.0 {
		t.Errorf("create text=%s fields=%v", text, fields)
	}
	if text := callAgile(t, handleUpdateTicket, conn, map[string]any{"ticket_key": "K-1", "custom_fields": map[string]any{"Nope": "x"}}); !strings.Contains(text, "\"Nope\": no Jira field") {
		t.Errorf("unknown field text = %s", text)
	}
}

func TestCreateRequest_FixVersions(t *testing.T) {
	got, err := createRequest(newCallToolRequest(map[string]any{"project": "P", "summary": "S", "fix_versions": "v1, v2"}))
	if err != nil || strings.Join(got.FixVersions, "|") != "v1|v2" {
		t.Errorf("got=%v err=%v", got.FixVersions, err)
	}
}
