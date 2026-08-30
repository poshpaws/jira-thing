package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// AgileBoardEndpoint and friends live under a separate base path (/rest/agile/1.0)
// from the rest of the Jira REST API.
const AgileBoardEndpoint = "/rest/agile/1.0/board"

// Board describes a Jira Agile board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type boardSearchResponse struct {
	Values []Board `json:"values"`
}

// FetchBoards returns the Agile boards visible to the user, optionally filtered
// to a single project.
func FetchBoards(conn JiraConnection, projectKey string) ([]Board, error) {
	endpoint := conn.BaseURL + AgileBoardEndpoint
	if projectKey != "" {
		endpoint += "?projectKeyOrId=" + projectKey
	}
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return nil, err
	}
	var result boardSearchResponse
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	return result.Values, nil
}

// Sprint describes a sprint on an Agile board.
type Sprint struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type sprintSearchResponse struct {
	Values []Sprint `json:"values"`
}

// FetchSprints returns the sprints on a board, optionally filtered by state
// (comma-separated: "active", "future", "closed").
func FetchSprints(conn JiraConnection, boardID int, state string) ([]Sprint, error) {
	endpoint := fmt.Sprintf("%s%s/%d/sprint", conn.BaseURL, AgileBoardEndpoint, boardID)
	if state != "" {
		endpoint += "?state=" + state
	}
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return nil, err
	}
	var result sprintSearchResponse
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	return result.Values, nil
}

// FetchSprintIssues returns the issues in a sprint, optionally narrowed by a JQL filter.
func FetchSprintIssues(conn JiraConnection, sprintID int, jql string) (SearchResult, error) {
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", conn.BaseURL, sprintID)
	if jql != "" {
		endpoint += "?jql=" + url.QueryEscape(jql)
	}
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return SearchResult{}, err
	}
	var result SearchResult
	return result, executeRequest(req, &result)
}

// AddIssuesToSprint moves up to 50 issues into a sprint.
func AddIssuesToSprint(conn JiraConnection, sprintID int, issueKeys []string) error {
	payload, err := json.Marshal(map[string]any{"issues": issueKeys})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", conn.BaseURL, sprintID)
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: endpoint,
		Body:     bytes.NewReader(payload),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
}
