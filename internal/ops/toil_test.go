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

// --- extractNotes ---

func TestExtractNotes_Found(t *testing.T) {
	body := "<!-- jira-thing:notes:start --><p>my notes</p><!-- jira-thing:notes:end -->"
	got := extractNotes(body)
	if got != "<p>my notes</p>" {
		t.Errorf("extractNotes = %q, want <p>my notes</p>", got)
	}
}

func TestExtractNotes_Missing(t *testing.T) {
	if got := extractNotes("<p>no markers</p>"); got != "" {
		t.Errorf("extractNotes = %q, want empty", got)
	}
}

// --- renderTicketPage ---

func testIssue(key, summary string) map[string]any {
	return map[string]any{
		"key": key,
		"fields": map[string]any{
			"summary":  summary,
			"updated":  "2026-05-20T10:00:00.000Z",
			"status":   map[string]any{"name": "Open"},
			"priority": map[string]any{"name": "High"},
		},
	}
}

func TestRenderTicketPage_ContainsDetailsAndNotes(t *testing.T) {
	out := renderTicketPage(testIssue("PROJ-1", "Fix toil"), "")
	for _, want := range []string{
		"jira-thing:details:start", "jira-thing:details:end",
		"jira-thing:notes:start", "jira-thing:notes:end",
		"ac:structured-macro", `ac:name="jira"`, "PROJ-1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestRenderTicketPage_PreservesExistingNotes(t *testing.T) {
	out := renderTicketPage(testIssue("PROJ-1", "Fix toil"), "<p>saved note</p>")
	if !strings.Contains(out, "<p>saved note</p>") {
		t.Errorf("notes not preserved: %s", out)
	}
}

func TestRenderTicketPage_KeyEscaped(t *testing.T) {
	out := renderTicketPage(testIssue(`<script>`, "irrelevant"), "")
	if strings.Contains(out, "<script>") {
		t.Errorf("unescaped key in output: %s", out)
	}
}

func TestRenderTicketPage_NewPageHasDateAndNotePanel(t *testing.T) {
	out := renderTicketPage(testIssue("PROJ-1", "Fix toil"), "")
	if !strings.Contains(out, `<time datetime=`) {
		t.Errorf("new page missing time element: %s", out)
	}
	if !strings.Contains(out, `ac:adf-extension`) {
		t.Errorf("new page missing adf-extension: %s", out)
	}
	if !strings.Contains(out, `panelType`) || !strings.Contains(out, `note`) {
		t.Errorf("new page missing note panelType: %s", out)
	}
}

func TestRenderTicketPage_ExistingNotesNoDatePanel(t *testing.T) {
	out := renderTicketPage(testIssue("PROJ-1", "Fix toil"), "<p>existing note</p>")
	if strings.Contains(out, `<time datetime=`) {
		t.Errorf("existing notes page should not inject date element: %s", out)
	}
	if strings.Contains(out, `ac:adf-extension`) {
		t.Errorf("existing notes page should not inject panel: %s", out)
	}
}

// --- renderHangerPage ---

func TestRenderHangerPage_WithIssues(t *testing.T) {
	issues := []map[string]any{
		testIssue("PROJ-1", "Fix toil"),
		testIssue("PROJ-2", "More toil"),
	}
	out := renderHangerPage(issues, nil)
	for _, want := range []string{"PROJ-1", "PROJ-2", "ac:link", "ri:page"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in hanger page: %s", want, out)
		}
	}
}

func TestRenderHangerPage_Empty(t *testing.T) {
	out := renderHangerPage([]map[string]any{}, nil)
	if !strings.Contains(out, "No open TOIL tickets") {
		t.Errorf("expected no-ticket message, got: %s", out)
	}
}

func TestRenderHangerPage_IncludesManualPages(t *testing.T) {
	issues := []map[string]any{testIssue("PROJ-1", "Fix toil")}
	manuals := []api.ConfluencePageWithBody{
		{ConfluencePage: api.ConfluencePage{Title: "Team Notes"}},
		{ConfluencePage: api.ConfluencePage{Title: "Process Guide"}},
	}
	out := renderHangerPage(issues, manuals)
	for _, want := range []string{"PROJ-1", "Team Notes", "Process Guide", "ac:link", "ri:page"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in hanger page: %s", want, out)
		}
	}
}

func TestValidateToilConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want error
	}{
		{"missing project", config.Config{ToilMarker: "T", ToilTeam: "X"}, ErrToilConfig},
		{"team field without team", config.Config{Project: "P", ToilMarker: "T", UseTeamField: true}, ErrToilTeamField},
		{"label without toil_team", config.Config{Project: "P", ToilMarker: "T"}, ErrToilTeamLabel},
		{"valid", config.Config{Project: "P", ToilMarker: "T", ToilTeam: "X"}, nil},
	}
	for _, tt := range tests {
		if got := ValidateToilConfig(tt.cfg); !errors.Is(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestToilJQL(t *testing.T) {
	label := ToilJQL(config.Config{Project: "P", ToilMarker: "T", ToilTeam: "X"}, "updated >= -1w")
	if label != `project = "P" AND labels = "T" AND labels = "X" AND updated >= -1w` {
		t.Errorf("label JQL = %s", label)
	}
	team := ToilJQL(config.Config{Project: "P", ToilMarker: "T", UseTeamField: true, Team: "u-1"}, "")
	if !strings.HasSuffix(team, `"Team[Team]" = u-1`) {
		t.Errorf("team JQL = %s", team)
	}
}

func TestFilterIssues(t *testing.T) {
	issues := []map[string]any{{"key": "A-1"}, {"key": "A-2"}}
	if got := FilterIssues(issues, nil); len(got) != 2 {
		t.Errorf("empty keys must keep all, got %v", got)
	}
	if got := FilterIssues(issues, []string{"A-2", "Z-9"}); len(got) != 1 || got[0]["key"] != "A-2" {
		t.Errorf("got %v", got)
	}
}

// toilConfluence fakes the hanger lookup, its children, and page writes.
type toilConfluence struct {
	children []map[string]any
	writes   []string
}

func (c *toilConfluence) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "child/page"):
		_ = json.NewEncoder(w).Encode(map[string]any{"results": c.children, "size": len(c.children)})
	case r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{
			"id": "99", "title": "Toil Tracker", "version": map[string]any{"number": float64(3)},
		}}, "size": 1})
	default:
		c.writes = append(c.writes, r.Method+" "+r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "1", "version": map[string]any{"number": float64(1)}})
	}
}

func TestSyncToilPages_CreatesUpdatesAndKeepsManualPages(t *testing.T) {
	c := &toilConfluence{children: []map[string]any{
		{"id": "7", "title": "PROJ-2", "version": map[string]any{"number": float64(1)},
			"body": map[string]any{"storage": map[string]any{"value": notesStart + "<p>kept</p>" + notesEnd}}},
		{"id": "8", "title": "Team Notes", "version": map[string]any{"number": float64(1)}},
	}}
	srv := httptest.NewTLSServer(c)
	t.Cleanup(srv.Close)
	conn := api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
	cfg := config.Config{ConfluenceSpace: "ENG", TicketHanger: "Toil Tracker"}

	res, err := SyncToilPages(conn, cfg, []map[string]any{testIssue("PROJ-1", "New"), testIssue("PROJ-2", "Old")})
	if err != nil {
		t.Fatal(err)
	}
	want := []PageSync{{"PROJ-1", PageCreated}, {"PROJ-2", PageUpdated}}
	if len(res.Pages) != 2 || res.Pages[0] != want[0] || res.Pages[1] != want[1] || res.Hanger != "Toil Tracker" {
		t.Errorf("res=%+v", res)
	}
	if len(c.writes) != 3 {
		t.Errorf("expected create, update and hanger rewrite, got %v", c.writes)
	}
}

func TestSyncToilPages_NeedsConfluenceConfig(t *testing.T) {
	if _, err := SyncToilPages(api.JiraConnection{}, config.Config{}, nil); !errors.Is(err, ErrToilConfluenceCfg) {
		t.Errorf("got %v", err)
	}
}
