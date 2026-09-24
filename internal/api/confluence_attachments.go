package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// maxConfluenceAttachmentSize is the maximum file size for Confluence attachments (50 MB).
const maxConfluenceAttachmentSize = 50 * 1024 * 1024

// maxConfluenceDownloadSize caps how much data a single attachment download will
// stream to disk, as a safety backstop against a misbehaving/malicious server.
const maxConfluenceDownloadSize = 200 * 1024 * 1024

// ConfluenceAttachment holds metadata for a Confluence attachment.
// DownloadPath and MediaType are populated by ListConfluenceAttachments (empty
// on upload responses, which don't return them) and are used to export/download
// the attachment's binary content.
type ConfluenceAttachment struct {
	ID           string
	Title        string
	DownloadPath string // relative path from _links.download, e.g. "/download/attachments/123/file.png?version=1"
	MediaType    string
}

// AddConfluenceAttachment uploads a local file as an attachment on a Confluence page.
// Returns the attachment metadata on success.
func AddConfluenceAttachment(conn JiraConnection, pageID, filePath string) (ConfluenceAttachment, error) {
	if err := validateNumericID(pageID, "page ID"); err != nil {
		return ConfluenceAttachment{}, err
	}
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
// Rejects files larger than maxConfluenceAttachmentSize to prevent OOM.
func buildConfluenceAttachmentBody(filePath string) (io.Reader, string, error) {
	file, err := os.Open(filePath) // #nosec G304 -- filePath is user-supplied CLI input
	if err != nil {
		return nil, "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > maxConfluenceAttachmentSize {
		return nil, "", fmt.Errorf("file %s is %d MB, exceeds %d MB limit",
			filepath.Base(filePath), info.Size()/(1024*1024), maxConfluenceAttachmentSize/(1024*1024))
	}

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

// ListConfluenceAttachments returns the existing attachments on a Confluence page,
// including each attachment's download path and media type.
func ListConfluenceAttachments(conn JiraConnection, pageID string) ([]ConfluenceAttachment, error) {
	if err := validateNumericID(pageID, "page ID"); err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s%s/%s/child/attachment?limit=100&expand=metadata.mediaType",
		conn.BaseURL, confluenceContentEndpoint, pageID)
	req, err := newAuthRequest(conn, APIRequest{Method: http.MethodGet, Endpoint: endpoint})
	if err != nil {
		return nil, err
	}
	var result struct {
		Results []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Metadata struct {
				MediaType string `json:"mediaType"`
			} `json:"metadata"`
			Links struct {
				Download string `json:"download"`
			} `json:"_links"`
		} `json:"results"`
	}
	if err := executeRequest(req, &result); err != nil {
		return nil, err
	}
	attachments := make([]ConfluenceAttachment, len(result.Results))
	for i, r := range result.Results {
		attachments[i] = ConfluenceAttachment{
			ID: r.ID, Title: r.Title,
			DownloadPath: r.Links.Download,
			MediaType:    r.Metadata.MediaType,
		}
	}
	return attachments, nil
}

// UpdateConfluenceAttachment replaces the data of an existing Confluence attachment.
// Uses POST /content/{pageID}/child/attachment/{attachmentID}/data.
func UpdateConfluenceAttachment(conn JiraConnection, pageID, attachmentID, filePath string) (ConfluenceAttachment, error) {
	if err := validateNumericID(pageID, "page ID"); err != nil {
		return ConfluenceAttachment{}, err
	}
	if err := validateNumericID(attachmentID, "attachment ID"); err != nil {
		return ConfluenceAttachment{}, err
	}
	body, contentType, err := buildConfluenceAttachmentBody(filePath)
	if err != nil {
		return ConfluenceAttachment{}, err
	}
	endpoint := fmt.Sprintf("%s%s/%s/child/attachment/%s/data",
		conn.BaseURL, confluenceContentEndpoint, pageID, attachmentID)
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
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := executeRequest(req, &result); err != nil {
		return ConfluenceAttachment{}, err
	}
	return ConfluenceAttachment{ID: result.ID, Title: result.Title}, nil
}

// DownloadConfluenceAttachment streams an attachment's binary content to destPath.
// downloadPath is the ConfluenceAttachment.DownloadPath returned by
// ListConfluenceAttachments (Confluence's own _links.download value) — never a
// caller-constructed path — so it is trusted as-is.
func DownloadConfluenceAttachment(conn JiraConnection, downloadPath, destPath string) error {
	if downloadPath == "" {
		return fmt.Errorf("attachment has no download link")
	}
	if err := ValidateConnection(conn); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, resolveConfluenceDownloadURL(conn.BaseURL, downloadPath), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(conn.Email, conn.APIToken)
	return streamAttachmentToFile(req, destPath)
}

// streamAttachmentToFile executes req and writes its response body to destPath,
// capped at maxConfluenceDownloadSize.
func streamAttachmentToFile(req *http.Request, destPath string) error {
	resp, err := httpClient.Do(req) // #nosec G107 -- URL built from conn.BaseURL (user's own config) + a Confluence-issued download path
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		return fmt.Errorf("downloading attachment: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	out, err := os.Create(destPath) // #nosec G304 -- destPath is built by the caller from a sanitised filename under a fixed output directory
	if err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(resp.Body, maxConfluenceDownloadSize)); err != nil {
		return fmt.Errorf("writing %s: %w", destPath, err)
	}
	return nil
}

// resolveConfluenceDownloadURL builds an absolute attachment download URL.
// Confluence's _links.download path is relative to the /wiki context root,
// while JiraConnection.BaseURL is the bare site root, so /wiki is prepended
// unless the path already carries it.
func resolveConfluenceDownloadURL(baseURL, downloadPath string) string {
	if strings.HasPrefix(downloadPath, "/wiki/") {
		return baseURL + downloadPath
	}
	return baseURL + "/wiki" + downloadPath
}

// DownloadConfluenceAttachmentsByFilename downloads every attachment on pageID
// whose filename appears in wanted, saving each to destDir. Filenames are
// sanitised to their base name before use as a destination path, so a crafted
// filename (e.g. containing "../") cannot escape destDir.
// Returns the filenames successfully downloaded and any wanted filenames with
// no matching attachment on the page, so callers can warn about dead references
// (e.g. an ri:attachment left over from a since-deleted attachment).
func DownloadConfluenceAttachmentsByFilename(conn JiraConnection, pageID string, wanted []string, destDir string) (downloaded, missing []string, err error) {
	if len(wanted) == 0 {
		return nil, nil, nil
	}
	attachments, err := ListConfluenceAttachments(conn, pageID)
	if err != nil {
		return nil, nil, err
	}
	byName := make(map[string]ConfluenceAttachment, len(attachments))
	for _, a := range attachments {
		byName[a.Title] = a
	}
	for _, filename := range wanted {
		att, ok := byName[filename]
		if !ok {
			missing = append(missing, filename)
			continue
		}
		destPath := filepath.Join(destDir, filepath.Base(filename))
		if dlErr := DownloadConfluenceAttachment(conn, att.DownloadPath, destPath); dlErr != nil {
			return downloaded, missing, fmt.Errorf("downloading %s: %w", filename, dlErr)
		}
		downloaded = append(downloaded, filename)
	}
	return downloaded, missing, nil
}
