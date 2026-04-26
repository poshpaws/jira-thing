package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	IssueEndpoint      = "/rest/api/3/issue"
	SearchEndpoint     = "/rest/api/3/search"
	requestTimeoutSecs = 30 * time.Second
)

// SearchResult holds the response from the Jira search API.
type SearchResult struct {
	Issues     []map[string]any `json:"issues"`
	Total      int              `json:"total"`
	MaxResults int              `json:"maxResults"`
}

// JiraConnection holds the connection details for the Jira API.
type JiraConnection struct {
	BaseURL  string
	Email    string
	APIToken string
}

var httpClient = &http.Client{Timeout: requestTimeoutSecs}

// FetchIssue retrieves a Jira issue by key.
func FetchIssue(conn JiraConnection, issueKey string) (map[string]any, error) {
	url := conn.BaseURL + IssueEndpoint + "/" + issueKey
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(conn.Email, conn.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SearchIssues executes a JQL search and returns matching issues.
// fields lists which fields to include (e.g. ["summary","status","priority","updated"]).
// maxResults caps the number of returned issues (Jira max is 100).
func SearchIssues(conn JiraConnection, jql string, fields []string, maxResults int) (SearchResult, error) {
	endpoint := conn.BaseURL + SearchEndpoint
	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	if len(fields) > 0 {
		params.Set("fields", strings.Join(fields, ","))
	}

	req, err := http.NewRequest(http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return SearchResult{}, err
	}
	req.SetBasicAuth(conn.Email, conn.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return SearchResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return SearchResult{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SearchResult{}, err
	}
	return result, nil
}

// CreateIssue creates a new Jira issue with the given fields.
func CreateIssue(conn JiraConnection, fields map[string]any) (map[string]any, error) {
	url := conn.BaseURL + IssueEndpoint

	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(conn.Email, conn.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SearchIssues executes a JQL search and returns matching issues.
// fields lists which fields to include (e.g. ["summary","status","priority","updated"]).
// maxResults caps the number of returned issues (Jira max is 100).
func SearchIssues(conn JiraConnection, jql string, fields []string, maxResults int) (SearchResult, error) {
	endpoint := conn.BaseURL + SearchEndpoint
	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	if len(fields) > 0 {
		params.Set("fields", strings.Join(fields, ","))
	}

	req, err := http.NewRequest(http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return SearchResult{}, err
	}
	req.SetBasicAuth(conn.Email, conn.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return SearchResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return SearchResult{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return SearchResult{}, err
	}
	return result, nil
}
