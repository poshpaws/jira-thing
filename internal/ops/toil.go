package ops

import (
	"errors"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// Toil page actions reported by SyncToilPages.
const (
	PageCreated = "created"
	PageUpdated = "updated"
)

// toilMaxResults caps toil searches; toil queues are small by nature.
const toilMaxResults = 100

// toilFields are the issue fields toil listings and pages need.
var toilFields = []string{"summary", "status", "priority", "updated"}

// Toil configuration errors, worded for the config file the user edits.
var (
	ErrToilConfig        = errors.New("project and toil_marker must be set in ~/.config/jira-thing/jira-thing.json")
	ErrToilTeamField     = errors.New("team must be set in ~/.config/jira-thing/jira-thing.json when use_team_field is true")
	ErrToilTeamLabel     = errors.New("toil_team must be set in ~/.config/jira-thing/jira-thing.json (or set use_team_field=true and team)")
	ErrToilConfluenceCfg = errors.New("confluence_space and ticket_hanger must be set in ~/.config/jira-thing/jira-thing.json")
)

// ValidateToilConfig checks the config fields toil queries need.
func ValidateToilConfig(cfg config.Config) error {
	switch {
	case cfg.Project == "" || cfg.ToilMarker == "":
		return ErrToilConfig
	case cfg.UseTeamField && cfg.Team == "":
		return ErrToilTeamField
	case !cfg.UseTeamField && cfg.ToilTeam == "":
		return ErrToilTeamLabel
	}
	return nil
}

// ToilJQL builds the toil ticket query, ANDing in extra when given. The team
// is matched on the Team field or, for older setups, a second label.
func ToilJQL(cfg config.Config, extra string) string {
	teamClause := fmt.Sprintf("labels = %q", cfg.ToilTeam)
	if cfg.UseTeamField {
		teamClause = fmt.Sprintf(`"Team[Team]" = %s`, cfg.Team)
	}
	jql := fmt.Sprintf("project = %q AND labels = %q AND %s", cfg.Project, cfg.ToilMarker, teamClause)
	if extra != "" {
		jql += " AND " + extra
	}
	return jql
}

// ListToilTickets returns the configured team's toil tickets matching extra JQL.
func ListToilTickets(conn api.JiraConnection, cfg config.Config, extra string) ([]map[string]any, error) {
	if err := ValidateToilConfig(cfg); err != nil {
		return nil, err
	}
	result, err := api.SearchIssues(conn, api.SearchQuery{JQL: ToilJQL(cfg, extra), Fields: toilFields, MaxResults: toilMaxResults})
	if err != nil {
		return nil, fmt.Errorf("fetching toil tickets: %w", err)
	}
	return result.Issues, nil
}

// OpenToilTickets returns the unresolved toil tickets, most recently updated first.
func OpenToilTickets(conn api.JiraConnection, cfg config.Config) ([]map[string]any, error) {
	return ListToilTickets(conn, cfg, "resolution = Unresolved ORDER BY updated DESC")
}

// FilterIssues returns the issues whose keys are in keys, or all of them when keys is empty.
func FilterIssues(issues []map[string]any, keys []string) []map[string]any {
	if len(keys) == 0 {
		return issues
	}
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	filtered := make([]map[string]any, 0, len(keys))
	for _, issue := range issues {
		if want[str(issue, "key")] {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// PageSync reports what happened to one ticket's Confluence page.
type PageSync struct {
	Key    string
	Action string
}

// ToilSyncResult reports a toil sync.
type ToilSyncResult struct {
	Space  string
	Hanger string
	Pages  []PageSync
}

// SyncToilPages writes a Confluence child page per issue under the configured
// hanger page, keeping each page's notes, then rewrites the hanger's index.
// Child pages that aren't ticket pages are kept in the index as manual pages.
func SyncToilPages(conn api.JiraConnection, cfg config.Config, issues []map[string]any) (ToilSyncResult, error) {
	if cfg.ConfluenceSpace == "" || cfg.TicketHanger == "" {
		return ToilSyncResult{}, ErrToilConfluenceCfg
	}
	hanger, err := api.FetchConfluencePage(conn, cfg.ConfluenceSpace, cfg.TicketHanger)
	if err != nil {
		return ToilSyncResult{}, err
	}
	children, err := api.ListChildPages(conn, hanger.ID)
	if err != nil {
		return ToilSyncResult{}, fmt.Errorf("listing child pages: %w", err)
	}
	res := ToilSyncResult{Space: cfg.ConfluenceSpace, Hanger: hanger.Title}
	byTitle := make(map[string]api.ConfluencePageWithBody, len(children))
	for _, c := range children {
		byTitle[c.Title] = c
	}
	for _, issue := range issues {
		page, err := syncTicketPage(conn, cfg.ConfluenceSpace, hanger.ID, issue, byTitle)
		if err != nil {
			return res, err
		}
		res.Pages = append(res.Pages, page)
	}
	index := renderHangerPage(issues, manualPages(children, res.Pages))
	if err := api.UpdateConfluencePage(conn, hanger.ID, hanger.Version, hanger.Title, index); err != nil {
		return res, fmt.Errorf("updating hanger page: %w", err)
	}
	return res, nil
}

// syncTicketPage creates or updates the child page for one issue.
func syncTicketPage(conn api.JiraConnection, space, parentID string, issue map[string]any,
	existing map[string]api.ConfluencePageWithBody) (PageSync, error) {
	key := str(issue, "key")
	body := renderTicketPage(issue, extractNotes(existing[key].Body))
	if child, ok := existing[key]; ok {
		if err := api.UpdateConfluencePage(conn, child.ID, child.Version, key, body); err != nil {
			return PageSync{}, fmt.Errorf("updating page %s: %w", key, err)
		}
		return PageSync{Key: key, Action: PageUpdated}, nil
	}
	if _, err := api.CreateConfluencePage(conn, space, key, parentID, body); err != nil {
		return PageSync{}, fmt.Errorf("creating page %s: %w", key, err)
	}
	return PageSync{Key: key, Action: PageCreated}, nil
}

// manualPages returns the child pages that aren't one of the synced ticket pages.
func manualPages(children []api.ConfluencePageWithBody, synced []PageSync) []api.ConfluencePageWithBody {
	ticket := make(map[string]bool, len(synced))
	for _, p := range synced {
		ticket[p.Key] = true
	}
	var manual []api.ConfluencePageWithBody
	for _, c := range children {
		if !ticket[c.Title] {
			manual = append(manual, c)
		}
	}
	return manual
}
