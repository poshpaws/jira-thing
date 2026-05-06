package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	IssueEndpoint  = "/rest/api/3/issue"
	SearchEndpoint = "/rest/api/3/search/jql"
	requestTimeout = 30 * time.Second
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

// APIRequest groups the HTTP method, endpoint URL, and optional body for a request.
type APIRequest struct {
	Method   string
	Endpoint string
	Body     io.Reader
}

// Comment represents a single Jira issue comment.
type Comment struct {
	Author  map[string]any `json:"author"`
	Body    map[string]any `json:"body"`
	Created string         `json:"created"`
}

// CommentResult holds the Jira comment list response.
type CommentResult struct {
	Comments []Comment `json:"comments"`
	Total    int       `json:"total"`
}

var httpClient = &http.Client{Timeout: requestTimeout}

// newAuthRequest creates an HTTP request with Basic Auth and Accept: application/json.
func newAuthRequest(conn JiraConnection, r APIRequest) (*http.Request, error) {
	req, err := http.NewRequest(r.Method, r.Endpoint, r.Body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(conn.Email, conn.APIToken)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// executeRequest sends req, asserts a 2xx status, and JSON-decodes the body into out.
// Pass nil for out when no response body is expected (e.g. 204 No Content).
func executeRequest(req *http.Request, out any) error {
	resp, err := httpClient.Do(req) // #nosec G704 -- URL originates from user's own OS keychain config, not external input
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// FetchIssue retrieves a single Jira issue by key.
func FetchIssue(conn JiraConnection, issueKey string) (map[string]any, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "?fields=*all",
	})
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
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + IssueEndpoint,
		Body:     bytes.NewReader(body),
	})
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var result map[string]any
	return result, executeRequest(req, &result)
}

// AddComment posts a comment on an existing Jira issue.
func AddComment(conn JiraConnection, issueKey string, body map[string]any) error {
	payload, err := json.Marshal(map[string]any{"body": body})
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/comment",
		Body:     bytes.NewReader(payload),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	var result map[string]any
	return executeRequest(req, &result)
}

// FetchLastComment retrieves the most recent comment on a Jira issue.
func FetchLastComment(conn JiraConnection, issueKey string) (Comment, error) {
	first, err := fetchCommentPage(conn, issueKey, 0, 1)
	if err != nil {
		return Comment{}, err
	}
	if first.Total == 0 {
		return Comment{}, fmt.Errorf("no comments on %s", issueKey)
	}
	if first.Total == 1 {
		return first.Comments[0], nil
	}
	page, err := fetchCommentPage(conn, issueKey, first.Total-1, 1)
	if err != nil {
		return Comment{}, err
	}
	if len(page.Comments) == 0 {
		return Comment{}, fmt.Errorf("no comments found")
	}
	return page.Comments[0], nil
}

// fetchCommentPage retrieves a page of comments from a Jira issue.
func fetchCommentPage(conn JiraConnection, issueKey string, startAt, maxResults int) (CommentResult, error) {
	endpoint := fmt.Sprintf("%s%s/%s/comment?startAt=%d&maxResults=%d",
		conn.BaseURL, IssueEndpoint, issueKey, startAt, maxResults)
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return CommentResult{}, err
	}
	var result CommentResult
	return result, executeRequest(req, &result)
}

// SearchIssues executes a JQL search via POST /rest/api/3/search/jql and returns matching issues.
func SearchIssues(conn JiraConnection, q SearchQuery) (SearchResult, error) {
	payload, err := json.Marshal(map[string]any{
		"jql":        q.JQL,
		"fields":     q.Fields,
		"maxResults": q.MaxResults,
	})
	if err != nil {
		return SearchResult{}, err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + SearchEndpoint,
		Body:     bytes.NewReader(payload),
	})
	if err != nil {
		return SearchResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var result SearchResult
	return result, executeRequest(req, &result)
}
