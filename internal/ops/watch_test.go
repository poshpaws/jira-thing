package ops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
)

// watchJira fakes myself, user search and the watchers endpoint, recording
// each mutating request.
type watchJira struct {
	sent []string
	fail bool
}

func (j *watchJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case j.fail:
		w.WriteHeader(http.StatusInternalServerError)
	case strings.HasSuffix(r.URL.Path, "/myself"):
		_ = json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me", "displayName": "Me"})
	case strings.HasSuffix(r.URL.Path, "/user/search"):
		_ = json.NewEncoder(w).Encode([]map[string]any{human("a1", "Ada", "ada@x.io")})
	case r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"watchers": []map[string]any{{"accountId": "a1", "displayName": "Ada"}}})
	default:
		j.sent = append(j.sent, r.Method+" "+r.URL.RawQuery)
		w.WriteHeader(http.StatusNoContent)
	}
}

func startWatch(t *testing.T, j *watchJira) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(j)
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func TestWatchTicket(t *testing.T) {
	tests := []struct {
		name       string
		req        WatchRequest
		dryRun     bool
		wantStatus string
		wantUser   string
		wantSent   string
	}{
		{"watch defaults to me", WatchRequest{Key: "K-1", Watch: true}, false, StatusWatching, "acc-me", "POST "},
		{"unwatch by name", WatchRequest{Key: "K-1", Who: "ada"}, false, StatusUnwatched, "a1", "DELETE accountId=a1"},
		{"picked", WatchRequest{Key: "K-1", Watch: true, Picked: &User{AccountID: "p1"}}, false, StatusWatching, "p1", "POST "},
		{"dry run", WatchRequest{Key: "K-1", Watch: true}, true, StatusDryRun, "acc-me", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &watchJira{}
			res, err := WatchTicket(startWatch(t, j), tt.req, tt.dryRun)
			sent := strings.Join(j.sent, ";")
			if err != nil || res.Status != tt.wantStatus || res.User.AccountID != tt.wantUser || sent != tt.wantSent {
				t.Errorf("res=%+v sent=%q err=%v", res, sent, err)
			}
		})
	}
}

func TestWatchTicket_Errors(t *testing.T) {
	conn := startWatch(t, &watchJira{fail: true})
	for _, watch := range []bool{true, false} {
		req := WatchRequest{Key: "K-1", Watch: watch, Picked: &User{AccountID: "p1"}}
		if _, err := WatchTicket(conn, req, false); err == nil || !strings.Contains(err.Error(), "watching K-1") {
			t.Errorf("watch=%v err=%v", watch, err)
		}
	}
	if _, err := WatchTicket(conn, WatchRequest{Key: "K-1", Who: "ada"}, false); err == nil {
		t.Error("expected lookup failure")
	}
}

func TestListWatchers(t *testing.T) {
	users, err := ListWatchers(startWatch(t, &watchJira{}), "K-1")
	if err != nil || len(users) != 1 || users[0].DisplayName != "Ada" {
		t.Errorf("users=%+v err=%v", users, err)
	}
	if _, err := ListWatchers(startWatch(t, &watchJira{fail: true}), "K-1"); err == nil {
		t.Error("expected error")
	}
}

func TestIsWatching(t *testing.T) {
	if watching, err := IsWatching(startWatch(t, &watchJira{}), "K-1"); err != nil || watching {
		t.Errorf("watching=%v err=%v", watching, err)
	}
	if _, err := IsWatching(startWatch(t, &watchJira{fail: true}), "K-1"); err == nil {
		t.Error("expected error")
	}
}

func TestWatchVerb(t *testing.T) {
	live, dry := WatchResult{Status: StatusWatching}, WatchResult{Status: StatusDryRun}
	got := []string{WatchVerb(live, true), WatchVerb(live, false), WatchVerb(dry, true), WatchVerb(dry, false)}
	if strings.Join(got, "|") != "now watching|no longer watching|would watch|would stop watching" {
		t.Errorf("verbs = %v", got)
	}
}
