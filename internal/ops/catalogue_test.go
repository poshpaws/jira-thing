package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// catalogueJira serves boards, projects and versions, or 500 when fail is set.
func catalogueJira(t *testing.T, fail bool) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var values []map[string]any
		switch {
		case fail:
			w.WriteHeader(http.StatusInternalServerError)
			return
		case strings.HasSuffix(r.URL.Path, "/version"):
			values = []map[string]any{
				{"name": "v3"}, {"name": "v2", "released": true}, {"name": "v1", "released": true, "archived": true},
			}
		case strings.HasSuffix(r.URL.Path, "/project/search"):
			values = []map[string]any{{"key": "PROJ", "name": "Project"}}
		default:
			values = []map[string]any{{"id": 1, "name": "B", "type": "simple"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"isLast": true, "values": values})
	}))
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func TestListReleases_Filters(t *testing.T) {
	tests := []struct {
		name   string
		filter ReleaseFilter
		want   string
	}{
		{"default hides archived", ReleaseFilter{Project: "PROJ"}, "v3,v2"},
		{"include archived", ReleaseFilter{Project: "PROJ", IncludeArchived: true}, "v3,v2,v1"},
		{"unreleased only", ReleaseFilter{Project: "PROJ", UnreleasedOnly: true}, "v3"},
	}
	conn := catalogueJira(t, false)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, err := ListReleases(conn, tt.filter)
			names := []string{}
			for _, v := range versions {
				names = append(names, v.Name)
			}
			if err != nil || strings.Join(names, ",") != tt.want {
				t.Errorf("names=%v err=%v", names, err)
			}
		})
	}
}

func TestListReleases_NeedsProject(t *testing.T) {
	if _, err := ListReleases(api.JiraConnection{}, ReleaseFilter{}); !errors.Is(err, ErrProjectRequired) {
		t.Errorf("err = %v", err)
	}
}

func TestListBoardsAndProjects(t *testing.T) {
	conn := catalogueJira(t, false)
	boards, err := ListBoards(conn, "PROJ")
	if err != nil || len(boards) != 1 {
		t.Errorf("boards=%+v err=%v", boards, err)
	}
	projects, err := ListProjects(conn)
	if err != nil || len(projects) != 1 || projects[0].Key != "PROJ" {
		t.Errorf("projects=%+v err=%v", projects, err)
	}
}

func TestCatalogue_WrapsErrors(t *testing.T) {
	conn := catalogueJira(t, true)
	if _, err := ListBoards(conn, ""); err == nil || !strings.Contains(err.Error(), "fetching boards") {
		t.Errorf("boards err = %v", err)
	}
	if _, err := ListProjects(conn); err == nil || !strings.Contains(err.Error(), "fetching projects") {
		t.Errorf("projects err = %v", err)
	}
	if _, err := ListReleases(conn, ReleaseFilter{Project: "PROJ"}); err == nil || !strings.Contains(err.Error(), "releases for PROJ") {
		t.Errorf("releases err = %v", err)
	}
}

func TestReleaseStatus(t *testing.T) {
	tests := []struct {
		v    api.ProjectVersion
		want string
	}{
		{api.ProjectVersion{Archived: true, Released: true}, ReleaseArchived},
		{api.ProjectVersion{Released: true}, ReleaseReleased},
		{api.ProjectVersion{Overdue: true}, ReleaseOverdue},
		{api.ProjectVersion{}, ReleaseUnreleased},
	}
	for _, tt := range tests {
		if got := ReleaseStatus(tt.v); got != tt.want {
			t.Errorf("ReleaseStatus(%+v) = %s, want %s", tt.v, got, tt.want)
		}
	}
}

func TestProjectOrConfig(t *testing.T) {
	cfg := config.Config{Project: "CFG"}
	if ProjectOrConfig(cfg, "") != "CFG" || ProjectOrConfig(cfg, "X") != "X" {
		t.Error("ProjectOrConfig fallback wrong")
	}
}
