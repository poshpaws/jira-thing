package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// JiraConnection holds the connection details for the Jira API.
type JiraConnection struct {
	BaseURL  string
	Email    string
	APIToken string
}

// SearchQuery holds the parameters for a JQL search request.
type SearchQuery struct {
	JQL        string
	Fields     []string
	MaxResults int
}

// SearchResult holds the response from the Jira search API.
type SearchResult struct {
	Issues     []map[string]any `json:"issues"`
	Total      int              `json:"total"`
	MaxResults int              `json:"maxResults"`
}

var httpClient = &http.Client{Timeout: requestTimeoutSecs}

// newAuthRequest creates an HTTP request with Basic Auth and Accept: application/json.
func newAuthRequest(conn JiraConnection, method, endpoint string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(conn.Email, conn.APIToken)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// executeRequest sends req, asserts a 2xx status, and JSON-decodes the body into out.
func executeRequest(req *http.Request, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// FetchIssue retrieves a single Jira issue by key.
func FetchIssue(conn JiraConnection, issueKey string) (map[string]any, error) {
	req, err := newAuthRequest(conn, http.MethodGet, conn.BaseURL+IssueEndpoint+"/"+issueKey, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	return result, executeRequest(req, &result)
}

// CreateIssue creates a new Jira issue with the provided fields payload.
func CreateIssue(conn JiraConnection, fields map[string]any) (map[string]any, error) {
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return nil, err
	}
	req, err := newAuthRequest(conn, http.MethodPost, conn.BaseURL+IssueEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var result map[string]any
	return result, executeRequest(req, &result)
}

// SearchIssues executes a JQL search and returns matching issues.
func SearchIssues(conn JiraConnection, q SearchQuery) (SearchResult, error) {
	params := url.Values{}
	params.Set("jql", q.JQL)
	params.Set("maxResults", fmt.Sprintf("%d", q.MaxResults))
	if len(q.Fields) > 0 {
		params.Set("fields", strings.Join(q.Fields, ","))
	}
	req, err := newAuthRequest(conn, http.MethodGet, conn.BaseURL+SearchEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return SearchResult{}, err
	}
	var result SearchResult
	return result, executeRequest(req, &result)
}
