package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
)

// Watchers is an issue's watcher list, as returned by GET .../watchers.
type Watchers struct {
	IsWatching bool   `json:"isWatching"`
	WatchCount int    `json:"watchCount"`
	Watchers   []User `json:"watchers"`
}

// watchersEndpoint is the watcher list URL for a validated issue key.
func watchersEndpoint(conn JiraConnection, issueKey string) (string, error) {
	if err := ValidateIssueKey(issueKey); err != nil {
		return "", err
	}
	return conn.BaseURL + IssueEndpoint + "/" + issueKey + "/watchers", nil
}

// FetchWatchers returns who is watching an issue.
func FetchWatchers(conn JiraConnection, issueKey string) (Watchers, error) {
	endpoint, err := watchersEndpoint(conn, issueKey)
	if err != nil {
		return Watchers{}, err
	}
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return Watchers{}, err
	}
	var result Watchers
	return result, executeRequest(req, &result)
}

// AddWatcher makes accountID watch an issue. Jira's body is the bare account
// ID as a JSON string.
func AddWatcher(conn JiraConnection, issueKey, accountID string) error {
	endpoint, err := watchersEndpoint(conn, issueKey)
	if err != nil {
		return err
	}
	body, err := json.Marshal(accountID)
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodPost, Endpoint: endpoint, Body: bytes.NewReader(body)})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
}

// RemoveWatcher stops accountID watching an issue.
func RemoveWatcher(conn JiraConnection, issueKey, accountID string) error {
	endpoint, err := watchersEndpoint(conn, issueKey)
	if err != nil {
		return err
	}
	endpoint += "?" + url.Values{"accountId": {accountID}}.Encode()
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodDelete, Endpoint: endpoint})
	if err != nil {
		return err
	}
	return executeRequest(req, nil)
}
