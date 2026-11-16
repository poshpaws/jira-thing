package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

// userJira fakes user search, myself and the assignee endpoint, recording
// each assignee PUT body.
type userJira struct {
	users   []map[string]any
	assigns []map[string]any
	fail    bool
}

func (j *userJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case j.fail:
		w.WriteHeader(http.StatusInternalServerError)
	case strings.HasSuffix(r.URL.Path, "/myself"):
		_ = json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me", "displayName": "Me"})
	case strings.HasSuffix(r.URL.Path, "/user/search"):
		_ = json.NewEncoder(w).Encode(j.users)
	case strings.HasSuffix(r.URL.Path, "/assignee"):
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		j.assigns = append(j.assigns, body)
		w.WriteHeader(http.StatusNoContent)
	}
}

func startUsers(t *testing.T, j *userJira) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(j)
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func human(id, name, email string) map[string]any {
	return map[string]any{"accountId": id, "displayName": name, "emailAddress": email, "active": true, "accountType": "atlassian"}
}

func TestSearchUsers_DropsInactiveAndApps(t *testing.T) {
	j := &userJira{users: []map[string]any{
		human("a1", "Ada", "ada@x.io"),
		{"accountId": "a2", "displayName": "Ada (gone)", "active": false, "accountType": "atlassian"},
		{"accountId": "a3", "displayName": "Ada Bot", "active": true, "accountType": "app"},
	}}
	users, err := SearchUsers(startUsers(t, j), "ada")
	if err != nil || len(users) != 1 || users[0].AccountID != "a1" {
		t.Errorf("users=%+v err=%v", users, err)
	}
}

func TestResolveUser(t *testing.T) {
	tests := []struct {
		name    string
		users   []map[string]any
		who     string
		wantID  string
		wantErr error
	}{
		{"me", nil, AssigneeMe, "acc-me", nil},
		{"account id skips search", nil, "5b10ac8d82e05b22cc7d4ef5", "5b10ac8d82e05b22cc7d4ef5", nil},
		{"single match", []map[string]any{human("a1", "Ada Lovelace", "")}, "ada", "a1", nil},
		{"exact name wins", []map[string]any{human("a1", "Ada", ""), human("a2", "Adam", "")}, "ADA", "a1", nil},
		{"exact email wins", []map[string]any{human("a1", "Ada", "ada@x.io"), human("a2", "Ada", "ada2@x.io")}, "ada2@x.io", "a2", nil},
		{"no match", nil, "zed", "", ErrUserNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ResolveUser(startUsers(t, &userJira{users: tt.users}), tt.who)
			if !errors.Is(err, tt.wantErr) || u.AccountID != tt.wantID {
				t.Errorf("user=%+v err=%v", u, err)
			}
		})
	}
}

func TestResolveUser_AmbiguousListsCandidates(t *testing.T) {
	j := &userJira{users: []map[string]any{human("a1", "Ada One", "one@x.io"), human("a2", "Ada Two", "")}}
	_, err := ResolveUser(startUsers(t, j), "ada")
	var amb *AmbiguousUserError
	if !errors.As(err, &amb) || len(amb.Candidates) != 2 {
		t.Fatalf("expected AmbiguousUserError, got %v", err)
	}
	if !strings.Contains(err.Error(), "Ada One <one@x.io> (a1)") || !strings.Contains(err.Error(), "Ada Two (a2)") {
		t.Errorf("message = %q", err.Error())
	}
}

func TestResolveUser_SearchFailureWrapped(t *testing.T) {
	_, err := ResolveUser(startUsers(t, &userJira{fail: true}), "ada")
	if err == nil || !strings.Contains(err.Error(), `searching users for "ada"`) {
		t.Errorf("got %v", err)
	}
}

func TestAssignTicket_Bodies(t *testing.T) {
	picked := User{AccountID: "p1", DisplayName: "Picked"}
	tests := []struct {
		name string
		req  AssignRequest
		want any
	}{
		{"name", AssignRequest{Key: "K-1", Assignee: "ada"}, "a1"},
		{"unassign", AssignRequest{Key: "K-1", Assignee: AssigneeUnassign}, nil},
		{"default", AssignRequest{Key: "K-1", Assignee: AssigneeDefault}, api.DefaultAssigneeID},
		{"picked", AssignRequest{Key: "K-1", Assignee: "ada", Picked: &picked}, "p1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &userJira{users: []map[string]any{human("a1", "Ada", "")}}
			res, err := AssignTicket(startUsers(t, j), tt.req, false)
			if err != nil || res.Status != StatusAssigned || len(j.assigns) != 1 {
				t.Fatalf("res=%+v err=%v", res, err)
			}
			if v, ok := j.assigns[0]["accountId"]; !ok || v != tt.want {
				t.Errorf("body = %v", j.assigns[0])
			}
		})
	}
}

func TestAssignTicket_DryRunAndErrors(t *testing.T) {
	j := &userJira{users: []map[string]any{human("a1", "Ada", "")}}
	conn := startUsers(t, j)
	res, err := AssignTicket(conn, AssignRequest{Key: "K-1", Assignee: "ada"}, true)
	if err != nil || res.Status != StatusDryRun || len(j.assigns) != 0 || *res.Payload["accountId"].(*string) != "a1" {
		t.Errorf("res=%+v err=%v", res, err)
	}
	if _, err := AssignTicket(conn, AssignRequest{Key: "K-1"}, false); !errors.Is(err, ErrAssigneeRequired) {
		t.Errorf("expected ErrAssigneeRequired, got %v", err)
	}
	failing := startUsers(t, &userJira{fail: true})
	if _, err := AssignTicket(failing, AssignRequest{Key: "K-1", Assignee: AssigneeUnassign}, false); err == nil ||
		!strings.Contains(err.Error(), "assigning K-1") {
		t.Errorf("got %v", err)
	}
}

func TestAssigneeLabel(t *testing.T) {
	tests := []struct {
		user User
		want string
	}{
		{User{}, "nobody (unassigned)"},
		{User{AccountID: api.DefaultAssigneeID}, "the project's default assignee"},
		{User{AccountID: "a1"}, "a1"},
		{User{AccountID: "a1", DisplayName: "Ada", Email: "ada@x.io"}, "Ada <ada@x.io> (a1)"},
	}
	for _, tt := range tests {
		if got := AssigneeLabel(tt.user); got != tt.want {
			t.Errorf("AssigneeLabel(%+v) = %q, want %q", tt.user, got, tt.want)
		}
	}
}
