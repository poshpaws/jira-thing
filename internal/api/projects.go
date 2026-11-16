package api

import (
	"fmt"
	"net/url"
	"regexp"
)

const (
	ProjectSearchEndpoint = "/rest/api/3/project/search"
	ProjectEndpoint       = "/rest/api/3/project"
)

// projectKeyPattern matches a Jira project key (PROJ) or numeric project ID.
var projectKeyPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*|[0-9]+)$`)

// ValidateProjectKey rejects anything that isn't a project key or numeric ID,
// since keys are interpolated into request paths.
func ValidateProjectKey(key string) error {
	if !projectKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid project key %q: expected a key like PROJ", key)
	}
	return nil
}

// Project describes a Jira project as returned by GET /rest/api/3/project/search.
type Project struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	ProjectTypeKey string `json:"projectTypeKey,omitempty"`
	Style          string `json:"style,omitempty"`
}

// FetchProjects returns every project the authenticated user can access.
func FetchProjects(conn JiraConnection) ([]Project, error) {
	return fetchAllPages[Project](conn, conn.BaseURL+ProjectSearchEndpoint, url.Values{})
}

// ProjectVersion describes a release/version on a project, as returned by
// GET /rest/api/3/project/{key}/version. Dates are YYYY-MM-DD, empty if unset.
type ProjectVersion struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Released    bool   `json:"released"`
	Archived    bool   `json:"archived"`
	Overdue     bool   `json:"overdue,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
}

// FetchProjectVersions returns every release/version configured on a project.
func FetchProjectVersions(conn JiraConnection, projectKey string) ([]ProjectVersion, error) {
	if err := ValidateProjectKey(projectKey); err != nil {
		return nil, err
	}
	endpoint := conn.BaseURL + ProjectEndpoint + "/" + projectKey + "/version"
	return fetchAllPages[ProjectVersion](conn, endpoint, url.Values{"orderBy": {"-releaseDate"}})
}
