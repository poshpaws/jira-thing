package api

import "net/http"

// EditField describes one field on an issue's edit screen, from GET .../editmeta.
type EditField struct {
	Name          string           `json:"name"`
	Schema        map[string]any   `json:"schema"`
	AllowedValues []map[string]any `json:"allowedValues"`
}

type editMetaResponse struct {
	Fields map[string]EditField `json:"fields"`
}

// FetchEditMeta returns the fields the current user can set on an issue with
// a plain edit, keyed by field ID. For issuetype, AllowedValues lists the
// types the issue can switch to without Jira's move flow.
func FetchEditMeta(conn JiraConnection, issueKey string) (map[string]EditField, error) {
	if err := ValidateIssueKey(issueKey); err != nil {
		return nil, err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/editmeta",
	})
	if err != nil {
		return nil, err
	}
	var result editMetaResponse
	return result.Fields, executeRequest(req, &result)
}
