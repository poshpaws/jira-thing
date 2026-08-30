package api

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// DeleteIssue permanently deletes an issue. When cascade is true, subtasks are
// deleted along with it; otherwise Jira rejects the delete if subtasks exist.
func DeleteIssue(conn JiraConnection, issueKey string, cascade bool) error {
	endpoint := conn.BaseURL + IssueEndpoint + "/" + issueKey + "?deleteSubtasks=false"
	if cascade {
		endpoint = conn.BaseURL + IssueEndpoint + "/" + issueKey + "?deleteSubtasks=true"
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodDelete,
		Endpoint: endpoint,
	})
	if err != nil {
		return err
	}
	return executeRequest(req, nil)
}

// AddWorklog logs time spent against an issue. timeSpent uses Jira's duration
// format (e.g. "2d 3h 30m"). comment is an optional ADF document body; pass nil
// to log time without a comment.
func AddWorklog(conn JiraConnection, issueKey, timeSpent string, comment map[string]any) error {
	payload := map[string]any{"timeSpent": timeSpent}
	if comment != nil {
		payload["comment"] = comment
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/worklog",
		Body:     bytes.NewReader(body),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	var result map[string]any
	return executeRequest(req, &result)
}
