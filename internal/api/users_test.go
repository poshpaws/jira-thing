package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jira-thing/internal/api"
)

func TestSearchUsers_SendsQueryAndDecodes(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != api.UserSearchEndpoint {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "ada l" || r.URL.Query().Get("maxResults") != "5" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"accountId": "a1", "displayName": "Ada Lovelace", "emailAddress": "ada@x.io", "active": true, "accountType": "atlassian"},
		})
	}))
	defer srv.Close()

	users, err := api.SearchUsers(conn(srv.URL), "ada l", 5)
	if err != nil {
		t.Fatal(err)
	}
	want := api.User{AccountID: "a1", DisplayName: "Ada Lovelace", EmailAddress: "ada@x.io", Active: true, AccountType: "atlassian"}
	if len(users) != 1 || users[0] != want {
		t.Errorf("users = %+v", users)
	}
}

func TestAssignIssue_Body(t *testing.T) {
	tests := []struct {
		name      string
		accountID *string
		want      any
	}{
		{"account", ptr("a1"), "a1"},
		{"unassign", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != api.IssueEndpoint+"/K-1/assignee" {
					t.Errorf("%s %s", r.Method, r.URL.Path)
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			if err := api.AssignIssue(conn(srv.URL), "K-1", tt.accountID); err != nil {
				t.Fatal(err)
			}
			if v, ok := body["accountId"]; !ok || v != tt.want {
				t.Errorf("body = %v", body)
			}
		})
	}
}

func TestAssignIssue_RejectsBadKey(t *testing.T) {
	if err := api.AssignIssue(conn("https://example.invalid"), "../x", nil); err == nil {
		t.Error("expected invalid key error")
	}
}

func ptr(s string) *string { return &s }
