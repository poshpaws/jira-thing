package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// UserSearchEndpoint finds users by display name or email.
const UserSearchEndpoint = "/rest/api/3/user/search"

// DefaultAssigneeID is the account ID Jira reads as "the project's default assignee".
const DefaultAssigneeID = "-1"

// User is a Jira user as returned by the user search API.
type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
	AccountType  string `json:"accountType"`
}

// SearchUsers returns up to maxResults users whose display name or email
// matches query. Jira hides emails the caller's privacy settings don't expose.
func SearchUsers(conn JiraConnection, query string, maxResults int) ([]User, error) {
	params := url.Values{"query": {query}, "maxResults": {strconv.Itoa(maxResults)}}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + UserSearchEndpoint + "?" + params.Encode(),
	})
	if err != nil {
		return nil, err
	}
	var users []User
	return users, executeRequest(req, &users)
}

// AssignIssue sets an issue's assignee. A nil accountID unassigns it, and
// DefaultAssigneeID hands it to the project's default assignee.
func AssignIssue(conn JiraConnection, issueKey string, accountID *string) error {
	if err := ValidateIssueKey(issueKey); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"accountId": accountID})
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPut,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/assignee",
		Body:     bytes.NewReader(body),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
}
