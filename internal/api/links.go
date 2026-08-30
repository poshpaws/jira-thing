package api

import (
	"bytes"
	"encoding/json"
	"net/http"
)

const (
	IssueLinkEndpoint     = "/rest/api/3/issueLink"
	IssueLinkTypeEndpoint = "/rest/api/3/issueLinkType"
)

// IssueLinkType describes a link type available on the Jira instance, as returned
// by GET /rest/api/3/issueLinkType. Link types are instance-configurable, so this
// list must always be read live rather than assumed (e.g. "Blocks" isn't guaranteed
// to exist on every board).
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// issueLinkTypesResponse wraps the Jira issueLinkType list endpoint response.
type issueLinkTypesResponse struct {
	IssueLinkTypes []IssueLinkType `json:"issueLinkTypes"`
}

// FetchIssueLinkTypes returns every issue link type configured on the Jira instance.
func FetchIssueLinkTypes(conn JiraConnection) ([]IssueLinkType, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + IssueLinkTypeEndpoint,
	})
	if err != nil {
		return nil, err
	}
	var result issueLinkTypesResponse
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	return result.IssueLinkTypes, nil
}

// LinkIssues creates a link between two issues using the named link type.
// outwardKey is the issue the outward description applies to (e.g. for type
// "Blocks", outwardKey "blocks" inwardKey).
func LinkIssues(conn JiraConnection, outwardKey, inwardKey, linkTypeName string) error {
	payload, err := json.Marshal(map[string]any{
		"type":         map[string]any{"name": linkTypeName},
		"outwardIssue": map[string]any{"key": outwardKey},
		"inwardIssue":  map[string]any{"key": inwardKey},
	})
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + IssueLinkEndpoint,
		Body:     bytes.NewReader(payload),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
}
