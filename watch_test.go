package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/tui"
)

// watchCLIHandler fakes myself, user search and watchers, recording mutations.
func watchCLIHandler(users, watchers []map[string]any, sent *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/myself"):
			json.NewEncoder(w).Encode(map[string]any{"accountId": "acc-me", "displayName": "Me"})
		case strings.HasSuffix(r.URL.Path, "/user/search"):
			json.NewEncoder(w).Encode(users)
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"watchers": watchers})
		default:
			*sent = append(*sent, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func startWatchCLI(t *testing.T, users, watchers []map[string]any) *[]string {
	t.Helper()
	sent := &[]string{}
	srv := httptest.NewTLSServer(watchCLIHandler(users, watchers, sent))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	return sent
}

func TestRunWatch(t *testing.T) {
	sent := startWatchCLI(t, nil, nil)
	out := captureStdout(func() { runWatch([]string{"PROJ-1"}) })
	if len(*sent) != 1 || !strings.HasPrefix((*sent)[0], "POST /rest/api/3/issue/PROJ-1/watchers") || !strings.Contains(out, "now watching") {
		t.Errorf("sent=%v out=%s", *sent, out)
	}
}

func TestRunUnwatch_ByNameJSON(t *testing.T) {
	sent := startWatchCLI(t, []map[string]any{cliUser("a1", "Ada")}, nil)
	out := captureStdout(func() { runUnwatch([]string{"PROJ-1", "ada", "-json"}) })
	var got watchJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Status != "unwatched" || got.AccountID != "a1" {
		t.Errorf("got=%+v out=%s", got, out)
	}
	if len(*sent) != 1 || !strings.Contains((*sent)[0], "DELETE") || !strings.Contains((*sent)[0], "accountId=a1") {
		t.Errorf("sent = %v", *sent)
	}
}

func TestRunWatch_DryRun(t *testing.T) {
	sent := startWatchCLI(t, nil, nil)
	out := captureStdout(func() { runWatch([]string{"PROJ-1", "-dry-run"}) })
	if len(*sent) != 0 || !strings.Contains(out, "Dry run:") || !strings.Contains(out, "would watch PROJ-1") {
		t.Errorf("sent=%v out=%s", *sent, out)
	}
}

func TestRunWatch_AmbiguousPicks(t *testing.T) {
	defer withInteractive(true)()
	old := selectMenuOptionFn
	defer func() { selectMenuOptionFn = old }()
	selectMenuOptionFn = func(string, []tui.MenuOption) (int, bool, error) { return 1, false, nil }
	sent := startWatchCLI(t, []map[string]any{cliUser("a1", "Ada One"), cliUser("a2", "Ada Two")}, nil)
	captureStdout(func() { runWatch([]string{"PROJ-1", "ada"}) })
	if len(*sent) != 1 {
		t.Errorf("sent = %v", *sent)
	}
}

func TestRunWatchers(t *testing.T) {
	startWatchCLI(t, nil, []map[string]any{{"accountId": "a1", "displayName": "Ada"}})
	if out := captureStdout(func() { runWatchers([]string{"PROJ-1"}) }); !strings.Contains(out, "Ada (a1)") {
		t.Errorf("out = %s", out)
	}
	if out := captureStdout(func() { runWatchers([]string{"PROJ-1", "-json"}) }); !strings.Contains(out, `"accountId":"a1"`) {
		t.Errorf("json out = %s", out)
	}
}

func TestRunWatchers_Nobody(t *testing.T) {
	startWatchCLI(t, nil, nil)
	if out := captureStdout(func() { runWatchers([]string{"PROJ-1"}) }); !strings.Contains(out, "Nobody is watching PROJ-1") {
		t.Errorf("out = %s", out)
	}
}

func TestWatch_Usage(t *testing.T) {
	cases := []func(){
		func() { runWatch(nil) }, func() { runUnwatch([]string{"A-1", "b", "c"}) }, func() { runWatchers(nil) },
		func() { runWatch([]string{"../x"}) },
	}
	startWatchCLI(t, nil, nil)
	for i, c := range cases {
		captureStderr(func() {
			if !captureExit(c) {
				t.Errorf("case %d: expected exit", i)
			}
		})
	}
}
