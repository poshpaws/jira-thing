package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// sprintJira fakes the Agile board, sprint and sprint-issue endpoints,
// recording the state asked for and each batch posted to a sprint.
type sprintJira struct {
	boards  []api.Board
	sprints []api.Sprint
	states  []string
	posted  [][]string
}

func (j *sprintJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost:
		var body struct{ Issues []string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		j.posted = append(j.posted, body.Issues)
		w.WriteHeader(http.StatusNoContent)
	case strings.HasSuffix(r.URL.Path, "/sprint"):
		j.states = append(j.states, r.URL.Query().Get("state"))
		_ = json.NewEncoder(w).Encode(map[string]any{"values": j.sprints, "isLast": true})
	case strings.HasSuffix(r.URL.Path, "/issue"):
		_ = json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{{"key": "PROJ-1"}}})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{"values": j.boards})
	}
}

func startSprints(t *testing.T, j *sprintJira) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(j)
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func TestScopeFromConfig(t *testing.T) {
	cfg := config.Config{Project: "CFG", Board: 7}
	tests := []struct {
		name    string
		board   int
		project string
		want    SprintScope
	}{
		{"config board and project", 0, "", SprintScope{BoardID: 7, Project: "CFG"}},
		{"explicit board", 3, "", SprintScope{BoardID: 3, Project: "CFG"}},
		{"explicit project ignores config board", 0, "OTH", SprintScope{Project: "OTH"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScopeFromConfig(cfg, tt.board, tt.project); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveBoard(t *testing.T) {
	scrum := func(id int) api.Board { return api.Board{ID: id, Name: "B", Type: "scrum"} }
	kanban := api.Board{ID: 9, Type: "kanban"}
	tests := []struct {
		name    string
		boards  []api.Board
		wantID  int
		wantErr func(error) bool
	}{
		{"only scrum board", []api.Board{kanban, scrum(4)}, 4, nil},
		{"team-managed board", []api.Board{kanban, {ID: 6, Type: "simple"}}, 6, nil},
		{"no sprint board", []api.Board{kanban}, 0, func(err error) bool { return errors.Is(err, ErrNoBoard) }},
		{"several sprint boards", []api.Board{scrum(1), scrum(2)}, 0, func(err error) bool {
			var choice *BoardChoiceError
			return errors.As(err, &choice) && strings.Contains(err.Error(), "[2] B")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board, err := ResolveBoard(startSprints(t, &sprintJira{boards: tt.boards}), SprintScope{Project: "PROJ"})
			if tt.wantErr != nil && !tt.wantErr(err) || tt.wantErr == nil && (err != nil || board.ID != tt.wantID) {
				t.Errorf("board=%+v err=%v", board, err)
			}
		})
	}
}

func TestResolveBoard_ExplicitIDSkipsLookup(t *testing.T) {
	board, err := ResolveBoard(api.JiraConnection{}, SprintScope{BoardID: 5})
	if err != nil || board.ID != 5 {
		t.Errorf("board=%+v err=%v", board, err)
	}
}

func TestFindSprint_Keywords(t *testing.T) {
	sprints := []api.Sprint{
		{ID: 1, StartDate: "2026-10-01", CompleteDate: "2026-10-14"},
		{ID: 2, StartDate: "2026-10-15", CompleteDate: "2026-10-28"},
	}
	tests := []struct {
		which, state string
		wantID       int
	}{
		{SprintCurrent, "active", 2},
		{SprintPrev, "closed", 2},
		{SprintNext, "future", 1},
	}
	for _, tt := range tests {
		t.Run(tt.which, func(t *testing.T) {
			j := &sprintJira{sprints: sprints}
			sp, err := FindSprint(startSprints(t, j), SprintScope{BoardID: 1}, tt.which)
			if err != nil || sp.ID != tt.wantID || j.states[0] != tt.state {
				t.Errorf("sprint=%+v states=%v err=%v", sp, j.states, err)
			}
		})
	}
}

func TestFindSprint_Errors(t *testing.T) {
	conn := startSprints(t, &sprintJira{})
	if _, err := FindSprint(conn, SprintScope{BoardID: 1}, "soon"); !errors.Is(err, ErrBadSprint) {
		t.Errorf("bad selector: err=%v", err)
	}
	if _, err := FindSprint(conn, SprintScope{BoardID: 1}, SprintCurrent); !errors.Is(err, ErrNoSprint) {
		t.Errorf("no sprint: err=%v", err)
	}
	if sp, err := FindSprint(api.JiraConnection{}, SprintScope{}, "42"); err != nil || sp.ID != 42 {
		t.Errorf("numeric: sprint=%+v err=%v", sp, err)
	}
}

func TestListSprintIssues(t *testing.T) {
	got, err := ListSprintIssues(startSprints(t, &sprintJira{}), SprintScope{}, "42", "")
	if err != nil || got.Sprint.ID != 42 || len(got.Issues) != 1 {
		t.Errorf("got=%+v err=%v", got, err)
	}
}

func TestAddToSprint_BatchesOf50(t *testing.T) {
	keys := make([]string, 0, 51)
	for i := 1; i <= 51; i++ {
		keys = append(keys, "PROJ-"+strconv.Itoa(i))
	}
	j := &sprintJira{}
	res, err := AddToSprint(startSprints(t, j), AddToSprintRequest{Sprint: "42", Keys: keys}, false)
	if err != nil || res.Status != StatusAdded || len(j.posted) != 2 || len(j.posted[0]) != 50 || len(j.posted[1]) != 1 {
		t.Errorf("res=%+v posted=%d err=%v", res, len(j.posted), err)
	}
}

func TestAddToSprint_DryRunSendsNothing(t *testing.T) {
	j := &sprintJira{}
	res, err := AddToSprint(startSprints(t, j), AddToSprintRequest{Sprint: "42", Keys: []string{"PROJ-1"}}, true)
	if err != nil || res.Status != StatusDryRun || len(j.posted) != 0 || res.Payload["issues"] == nil {
		t.Errorf("res=%+v posted=%v err=%v", res, j.posted, err)
	}
}

func TestAddToSprint_Rejects(t *testing.T) {
	tests := []struct {
		name string
		req  AddToSprintRequest
		want error
	}{
		{"no keys", AddToSprintRequest{Sprint: "42"}, ErrNoIssueKeys},
		{"closed sprint", AddToSprintRequest{Sprint: SprintPrev, Keys: []string{"PROJ-1"}}, ErrClosedSprintTarget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := AddToSprint(api.JiraConnection{}, tt.req, false); !errors.Is(err, tt.want) {
				t.Errorf("err=%v, want %v", err, tt.want)
			}
		})
	}
	if _, err := AddToSprint(api.JiraConnection{}, AddToSprintRequest{Sprint: "42", Keys: []string{"bad key"}}, false); err == nil {
		t.Error("expected bad key to be rejected")
	}
}

func TestCreateSprintScope(t *testing.T) {
	cfg := config.Config{Project: "PROJ", Board: 9}
	tests := []struct {
		name    string
		board   int
		project string
		want    SprintScope
	}{
		{"explicit board wins", 3, "PROJ", SprintScope{BoardID: 3, Project: "PROJ"}},
		{"configured project uses configured board", 0, "PROJ", SprintScope{BoardID: 9, Project: "PROJ"}},
		{"other project ignores configured board", 0, "OTHER", SprintScope{Project: "OTHER"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateSprintScope(cfg, tt.board, tt.project); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
