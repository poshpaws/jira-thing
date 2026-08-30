package api

import "net/http"

const (
	ProjectSearchEndpoint = "/rest/api/3/project/search"
	ProjectEndpoint       = "/rest/api/3/project"
)

// Project describes a Jira project as returned by GET /rest/api/3/project/search.
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type projectSearchResponse struct {
	Values []Project `json:"values"`
}

// FetchProjects returns every project the authenticated user can access.
func FetchProjects(conn JiraConnection) ([]Project, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + ProjectSearchEndpoint,
	})
	if err != nil {
		return nil, err
	}
	var result projectSearchResponse
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	return result.Values, nil
}

// ProjectVersion describes a release/version on a project, as returned by
// GET /rest/api/3/project/{key}/version.
type ProjectVersion struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Released bool   `json:"released"`
	Archived bool   `json:"archived"`
}

type projectVersionsResponse struct {
	Values []ProjectVersion `json:"values"`
}

// FetchProjectVersions returns the releases/versions configured on a project.
func FetchProjectVersions(conn JiraConnection, projectKey string) ([]ProjectVersion, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + ProjectEndpoint + "/" + projectKey + "/version",
	})
	if err != nil {
		return nil, err
	}
	var result projectVersionsResponse
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	return result.Values, nil
}
