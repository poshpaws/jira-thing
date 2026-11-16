package ops

import (
	"errors"
	"fmt"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// Release statuses reported by ReleaseStatus.
const (
	ReleaseArchived   = "archived"
	ReleaseReleased   = "released"
	ReleaseOverdue    = "overdue"
	ReleaseUnreleased = "unreleased"
)

// ErrProjectRequired means a project-scoped list had no project, given or configured.
var ErrProjectRequired = errors.New(`project key required: pass one or set "project" in config`)

// ProjectOrConfig returns project, or the configured project when it is empty.
func ProjectOrConfig(cfg config.Config, project string) string {
	if project != "" {
		return project
	}
	return cfg.Project
}

// ListBoards returns the Agile boards visible to the user, optionally only a
// project's.
func ListBoards(conn api.JiraConnection, project string) ([]api.Board, error) {
	boards, err := api.FetchBoards(conn, project)
	if err != nil {
		return nil, fmt.Errorf("fetching boards: %w", err)
	}
	return boards, nil
}

// ListProjects returns every project the user can browse.
func ListProjects(conn api.JiraConnection) ([]api.Project, error) {
	projects, err := api.FetchProjects(conn)
	if err != nil {
		return nil, fmt.Errorf("fetching projects: %w", err)
	}
	return projects, nil
}

// ReleaseFilter picks a project's releases. Archived releases are left out
// unless IncludeArchived; UnreleasedOnly also drops released ones.
type ReleaseFilter struct {
	Project         string
	IncludeArchived bool
	UnreleasedOnly  bool
}

// ListReleases returns a project's releases, newest release date first.
func ListReleases(conn api.JiraConnection, f ReleaseFilter) ([]api.ProjectVersion, error) {
	if f.Project == "" {
		return nil, ErrProjectRequired
	}
	versions, err := api.FetchProjectVersions(conn, f.Project)
	if err != nil {
		return nil, fmt.Errorf("fetching releases for %s: %w", f.Project, err)
	}
	kept := []api.ProjectVersion{}
	for _, v := range versions {
		if f.keeps(v) {
			kept = append(kept, v)
		}
	}
	return kept, nil
}

// keeps reports whether v passes the filter.
func (f ReleaseFilter) keeps(v api.ProjectVersion) bool {
	if v.Archived && !f.IncludeArchived {
		return false
	}
	return !(v.Released && f.UnreleasedOnly)
}

// ReleaseStatus is one word for a release's state, for both front ends.
func ReleaseStatus(v api.ProjectVersion) string {
	switch {
	case v.Archived:
		return ReleaseArchived
	case v.Released:
		return ReleaseReleased
	case v.Overdue:
		return ReleaseOverdue
	}
	return ReleaseUnreleased
}
