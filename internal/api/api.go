package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	IssueEndpoint       = "/rest/api/3/issue"
	SearchEndpoint      = "/rest/api/3/search/jql"
	MyselfEndpoint      = "/rest/api/3/myself"
	FieldEndpoint       = "/rest/api/3/field"
	requestTimeout      = 30 * time.Second
	maxErrBody          = 4096
	maxResponseBody     = 50 * 1024 * 1024 // 50MB limit on Jira response bodies
	maxAttachmentUpload = 50 * 1024 * 1024 // 50MB limit on attachment uploads
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

var httpClient = &http.Client{
	Timeout: requestTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return validateRedirect(req)
	},
}

// SetHTTPClient replaces the package-level HTTP client. Used in tests to inject
// a client that accepts self-signed TLS certificates from httptest.NewTLSServer.
func SetHTTPClient(c *http.Client) {
	httpClient = c
}

// validateRedirect rejects redirects to non-HTTPS URLs or URLs with embedded credentials.
func validateRedirect(req *http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect to non-HTTPS URL: %s", req.URL.Redacted())
	}
	if req.URL.User != nil {
		return fmt.Errorf("refusing redirect to URL with embedded credentials")
	}
	return nil
}

// ValidateConnection validates that a Jira connection uses HTTPS and has no embedded credentials.
func ValidateConnection(conn JiraConnection) error {
	parsed, err := url.Parse(conn.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid Jira base URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("jira base URL must use HTTPS, got %q", parsed.Scheme)
	}
	if parsed.User != nil {
		return fmt.Errorf("jira base URL must not contain embedded credentials")
	}
	return nil
}

// newAuthRequest creates an HTTP request with Basic Auth and Accept: application/json.
func newAuthRequest(conn JiraConnection, r APIRequest) (*http.Request, error) {
	if err := ValidateConnection(conn); err != nil {
		return nil, err
	}
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
	resp, err := httpClient.Do(req) // #nosec G107 -- URL originates from user's own OS keychain config, not external input
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		if len(bytes.TrimSpace(body)) > 0 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(body))
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxResponseBody)).Decode(out)
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

// UpdateIssue edits fields on an existing Jira issue (e.g. summary, description,
// priority, labels, assignee). Only the fields present in the map are changed.
func UpdateIssue(conn JiraConnection, issueKey string, fields map[string]any) error {
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPut,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey,
		Body:     bytes.NewReader(body),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
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

// AddAttachment uploads a local file as an attachment on an existing Jira issue.
func AddAttachment(conn JiraConnection, issueKey, filePath string) ([]map[string]any, error) {
	body, contentType, err := buildAttachmentBody(filePath)
	if err != nil {
		return nil, err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/attachments",
		Body:     body,
	})
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Atlassian-Token", "no-check")
	var result []map[string]any
	return result, executeRequest(req, &result)
}

// buildAttachmentBody reads filePath and returns a multipart/form-data body plus its Content-Type.
func buildAttachmentBody(filePath string) (io.Reader, string, error) {
	file, err := os.Open(filePath) // #nosec G304 -- filePath is user-supplied CLI input
	if err != nil {
		return nil, "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > maxAttachmentUpload {
		return nil, "", fmt.Errorf("attachment %s exceeds %d MB upload limit", filepath.Base(filePath), maxAttachmentUpload/(1024*1024))
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, "", fmt.Errorf("building attachment form: %w", err)
	}
	if _, err := io.Copy(part, io.LimitReader(file, maxAttachmentUpload)); err != nil {
		return nil, "", fmt.Errorf("reading file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("closing multipart writer: %w", err)
	}
	return &buf, writer.FormDataContentType(), nil
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

// Transition describes a single workflow transition available on an issue,
// as returned by GET /rest/api/3/issue/{key}/transitions.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"to"`
}

// transitionsResponse wraps the Jira transitions list endpoint response.
type transitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

// FetchTransitions returns the workflow transitions currently available on an issue.
// Boards customise workflows heavily, so this list (and the target states it names)
// must always be read live rather than assumed.
func FetchTransitions(conn JiraConnection, issueKey string) ([]Transition, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/transitions",
	})
	if err != nil {
		return nil, err
	}
	var result transitionsResponse
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	return result.Transitions, nil
}

// TransitionOptions holds the optional side effects a workflow transition can apply
// in the same call: field changes (e.g. resolution, assignee) and/or a comment.
// Some workflows require these to be set during the transition rather than after.
type TransitionOptions struct {
	Fields  map[string]any
	Comment map[string]any // ADF document body; nil to add no comment
}

// TransitionIssue moves an issue through its workflow by executing the transition
// identified by transitionID (from FetchTransitions).
func TransitionIssue(conn JiraConnection, issueKey, transitionID string) error {
	return TransitionIssueWithOptions(conn, issueKey, transitionID, TransitionOptions{})
}

// TransitionIssueWithOptions is TransitionIssue with optional field changes and/or
// a comment applied atomically as part of the transition.
func TransitionIssueWithOptions(conn JiraConnection, issueKey, transitionID string, opts TransitionOptions) error {
	payload := map[string]any{"transition": map[string]any{"id": transitionID}}
	if len(opts.Fields) > 0 {
		payload["fields"] = opts.Fields
	}
	if opts.Comment != nil {
		payload["update"] = map[string]any{
			"comment": []map[string]any{{"add": map[string]any{"body": opts.Comment}}},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/transitions",
		Body:     bytes.NewReader(body),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
}

// Field describes a Jira field as returned by GET /rest/api/3/field.
type Field struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
}

// FetchFields retrieves every field known to the Jira instance (system and custom).
func FetchFields(conn JiraConnection) ([]Field, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + FieldEndpoint,
	})
	if err != nil {
		return nil, err
	}
	var result []Field
	return result, executeRequest(req, &result)
}

// FetchMyself returns the currently authenticated Jira user's account details.
func FetchMyself(conn JiraConnection) (map[string]any, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + MyselfEndpoint,
	})
	if err != nil {
		return nil, err
	}
	var result map[string]any
	return result, executeRequest(req, &result)
}
