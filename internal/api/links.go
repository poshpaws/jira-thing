package api

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// issueLinksResponse decodes just the issuelinks field of a GET issue response.
type issueLinksResponse struct {
	Fields struct {
		IssueLinks []struct {
			ID           string `json:"id"`
			OutwardIssue *struct {
				Key string `json:"key"`
			} `json:"outwardIssue,omitempty"`
			InwardIssue *struct {
				Key string `json:"key"`
			} `json:"inwardIssue,omitempty"`
		} `json:"issuelinks"`
	} `json:"fields"`
}

// UnlinkIssues removes the link between two issues, whichever link type connects them.
func UnlinkIssues(conn JiraConnection, issueKey, otherKey string) error {
	linkID, err := findIssueLinkID(conn, issueKey, otherKey)
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodDelete,
		Endpoint: conn.BaseURL + IssueLinkEndpoint + "/" + linkID,
	})
	if err != nil {
		return err
	}
	return executeRequest(req, nil)
}

// findIssueLinkID looks up the link ID connecting issueKey to otherKey.
func findIssueLinkID(conn JiraConnection, issueKey, otherKey string) (string, error) {
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodGet,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "?fields=issuelinks",
	})
	if err != nil {
		return "", err
	}
	var result issueLinksResponse
	if err := executeRequest(req, &result); err != nil {
		return "", err
	}
	for _, link := range result.Fields.IssueLinks {
		if link.OutwardIssue != nil && link.OutwardIssue.Key == otherKey {
			return link.ID, nil
		}
		if link.InwardIssue != nil && link.InwardIssue.Key == otherKey {
			return link.ID, nil
		}
	}
	return "", fmt.Errorf("no link found between %s and %s", issueKey, otherKey)
}

// AddRemoteLink attaches a remote web link (e.g. a URL to external documentation) to an issue.
func AddRemoteLink(conn JiraConnection, issueKey, url, title string) error {
	payload, err := json.Marshal(map[string]any{
		"object": map[string]any{"url": url, "title": title},
	})
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + IssueEndpoint + "/" + issueKey + "/remotelink",
		Body:     bytes.NewReader(payload),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
}
