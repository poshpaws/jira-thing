package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// AgileBoardEndpoint and friends live under a separate base path (/rest/agile/1.0)
// from the rest of the Jira REST API.
const AgileBoardEndpoint = "/rest/agile/1.0/board"

// MaxSprintIssues is the most issues Jira accepts in one move-to-sprint call.
const MaxSprintIssues = 50

// sprintIssueLimit caps the issues fetched for one sprint.
const sprintIssueLimit = 100

// sprintIssueFields are the issue fields the sprint issue list asks for.
const sprintIssueFields = "summary,status,priority,updated,assignee,issuetype"

// Board describes a Jira Agile board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// FetchBoards returns every Agile board visible to the user, optionally
// filtered to a single project.
func FetchBoards(conn JiraConnection, projectKey string) ([]Board, error) {
	params := url.Values{}
	if projectKey != "" {
		params.Set("projectKeyOrId", projectKey)
	}
	return fetchAllPages[Board](conn, conn.BaseURL+AgileBoardEndpoint, params)
}

// Sprint describes a sprint on an Agile board. Dates are ISO 8601 strings and
// are empty until the sprint is started (or, for CompleteDate, closed).
type Sprint struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	State        string `json:"state"`
	StartDate    string `json:"startDate,omitempty"`
	EndDate      string `json:"endDate,omitempty"`
	CompleteDate string `json:"completeDate,omitempty"`
	Goal         string `json:"goal,omitempty"`
}

// FetchSprints returns every sprint on a board, oldest first, optionally
// filtered by state (comma-separated: "active", "future", "closed").
func FetchSprints(conn JiraConnection, boardID int, state string) ([]Sprint, error) {
	params := url.Values{}
	if state != "" {
		params.Set("state", state)
	}
	return fetchAllPages[Sprint](conn, fmt.Sprintf("%s%s/%d/sprint", conn.BaseURL, AgileBoardEndpoint, boardID), params)
}

// FetchSprintIssues returns up to 100 issues in a sprint, optionally narrowed
// by a JQL filter.
func FetchSprintIssues(conn JiraConnection, sprintID int, jql string) (SearchResult, error) {
	params := url.Values{"fields": {sprintIssueFields}, "maxResults": {strconv.Itoa(sprintIssueLimit)}}
	if jql != "" {
		params.Set("jql", jql)
	}
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue?%s", conn.BaseURL, sprintID, params.Encode())
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return SearchResult{}, err
	}
	var result SearchResult
	return result, executeRequest(req, &result)
}

// AddIssuesToSprint moves up to MaxSprintIssues issues into a sprint.
func AddIssuesToSprint(conn JiraConnection, sprintID int, issueKeys []string) error {
	for _, key := range issueKeys {
		if err := ValidateIssueKey(key); err != nil {
			return err
		}
	}
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
