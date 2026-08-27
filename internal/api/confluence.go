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
)

const confluenceContentEndpoint = "/wiki/rest/api/content"

// ConfluencePage holds the metadata needed to identify and update a Confluence page.
type ConfluencePage struct {
	ID      string
	Title   string
	Version int
}

// FetchConfluencePage finds a Confluence page by space key and title.
// Returns an error if the page does not exist.
func FetchConfluencePage(conn JiraConnection, space, title string) (ConfluencePage, error) {
	endpoint := conn.BaseURL + confluenceContentEndpoint +
		"?spaceKey=" + url.QueryEscape(space) +
		"&title=" + url.QueryEscape(title) +
		"&expand=version"
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return ConfluencePage{}, err
	}
	var result struct {
		Results []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
		} `json:"results"`
		Size int `json:"size"`
	}
	if err := executeRequest(req, &result); err != nil {
		return ConfluencePage{}, err
	}
	if result.Size == 0 || len(result.Results) == 0 {
		return ConfluencePage{}, fmt.Errorf("confluence page %q not found in space %q; create it manually first", title, space)
	}
	r := result.Results[0]
	return ConfluencePage{ID: r.ID, Title: r.Title, Version: r.Version.Number}, nil
}

// FetchConfluencePageByID retrieves a single Confluence page by its numeric ID.
func FetchConfluencePageByID(conn JiraConnection, pageID string) (ConfluencePage, error) {
	endpoint := fmt.Sprintf("%s%s/%s?expand=version", conn.BaseURL, confluenceContentEndpoint, pageID)
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return ConfluencePage{}, err
	}
	var result struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
	}
	if err := executeRequest(req, &result); err != nil {
		return ConfluencePage{}, err
	}
	return ConfluencePage{ID: result.ID, Title: result.Title, Version: result.Version.Number}, nil
}

// ConfluencePageWithBody extends ConfluencePage with the page's current storage body.
type ConfluencePageWithBody struct {
	ConfluencePage
	Body string
}

// ListChildPages returns all direct child pages of parentID, including their storage body.
func ListChildPages(conn JiraConnection, parentID string) ([]ConfluencePageWithBody, error) {
	endpoint := fmt.Sprintf("%s%s/%s/child/page?expand=body.storage,version&limit=100",
		conn.BaseURL, confluenceContentEndpoint, parentID)
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return nil, err
	}
	var result struct {
		Results []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
			Body struct {
				Storage struct {
					Value string `json:"value"`
				} `json:"storage"`
			} `json:"body"`
		} `json:"results"`
	}
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	pages := make([]ConfluencePageWithBody, len(result.Results))
	for i, r := range result.Results {
		pages[i] = ConfluencePageWithBody{
			ConfluencePage: ConfluencePage{ID: r.ID, Title: r.Title, Version: r.Version.Number},
			Body:           r.Body.Storage.Value,
		}
	}
	return pages, nil
}

// ListChildPagesSummary returns direct child pages of parentID without their body content.
// Lighter than ListChildPages — used by the space browser TUI where only titles are needed.
func ListChildPagesSummary(conn JiraConnection, parentID string) ([]ConfluencePage, error) {
	endpoint := fmt.Sprintf("%s%s/%s/child/page?expand=version&limit=100",
		conn.BaseURL, confluenceContentEndpoint, parentID)
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return nil, err
	}
	var result struct {
		Results []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
		} `json:"results"`
	}
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	pages := make([]ConfluencePage, len(result.Results))
	for i, r := range result.Results {
		pages[i] = ConfluencePage{ID: r.ID, Title: r.Title, Version: r.Version.Number}
	}
	return pages, nil
}

// CreateConfluencePage creates a new child page under parentID in the given space.
func CreateConfluencePage(conn JiraConnection, spaceKey, title, parentID, body string) (ConfluencePage, error) {
	payload, err := json.Marshal(map[string]any{
		"type":      "page",
		"title":     title,
		"space":     map[string]any{"key": spaceKey},
		"ancestors": []map[string]any{{"id": parentID}},
		"body": map[string]any{
			"storage": map[string]any{
				"value":          body,
				"representation": "storage",
			},
		},
	})
	if err != nil {
		return ConfluencePage{}, err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: conn.BaseURL + confluenceContentEndpoint,
		Body:     bytes.NewReader(payload),
	})
	if err != nil {
		return ConfluencePage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var result struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
	}
	if err := executeRequest(req, &result); err != nil {
		return ConfluencePage{}, err
	}
	return ConfluencePage{ID: result.ID, Title: result.Title, Version: result.Version.Number}, nil
}

// UpdateConfluencePage replaces the storage-format body of a Confluence page,
// incrementing its version number.
func UpdateConfluencePage(conn JiraConnection, id string, version int, title, body string) error {
	payload, err := json.Marshal(map[string]any{
		"version": map[string]any{"number": version + 1},
		"title":   title,
		"type":    "page",
		"body": map[string]any{
			"storage": map[string]any{
				"value":          body,
				"representation": "storage",
			},
		},
	})
	if err != nil {
		return err
	}
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPut,
		Endpoint: fmt.Sprintf("%s%s/%s", conn.BaseURL, confluenceContentEndpoint, id),
		Body:     bytes.NewReader(payload),
	})
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return executeRequest(req, nil)
}

// ConfluenceAttachment holds metadata for an uploaded Confluence attachment.
type ConfluenceAttachment struct {
	ID    string
	Title string
}

// AddConfluenceAttachment uploads a local file as an attachment on a Confluence page.
// Returns the attachment metadata on success.
func AddConfluenceAttachment(conn JiraConnection, pageID, filePath string) (ConfluenceAttachment, error) {
	body, contentType, err := buildConfluenceAttachmentBody(filePath)
	if err != nil {
		return ConfluenceAttachment{}, err
	}
	endpoint := fmt.Sprintf("%s%s/%s/child/attachment", conn.BaseURL, confluenceContentEndpoint, pageID)
	req, err := newAuthRequest(conn, APIRequest{
		Method:   http.MethodPost,
		Endpoint: endpoint,
		Body:     body,
	})
	if err != nil {
		return ConfluenceAttachment{}, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Atlassian-Token", "nocheck")
	var result struct {
		Results []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"results"`
	}
	if err := executeRequest(req, &result); err != nil {
		return ConfluenceAttachment{}, err
	}
	if len(result.Results) == 0 {
		return ConfluenceAttachment{}, fmt.Errorf("attachment upload returned no results")
	}
	r := result.Results[0]
	return ConfluenceAttachment{ID: r.ID, Title: r.Title}, nil
}

// buildConfluenceAttachmentBody creates a multipart/form-data body for Confluence attachment upload.
func buildConfluenceAttachmentBody(filePath string) (io.Reader, string, error) {
	file, err := os.Open(filePath) // #nosec G304 -- filePath is user-supplied CLI input
	if err != nil {
		return nil, "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, "", fmt.Errorf("building attachment form: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("reading file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("closing multipart writer: %w", err)
	}
	return &buf, writer.FormDataContentType(), nil
}
