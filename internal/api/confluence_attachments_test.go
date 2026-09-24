package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// httpClient is already pointed at a self-signed-cert-tolerant client by
// confluence_test.go's init() in this package, shared across all test files.

func TestListConfluenceAttachments_ReturnsDownloadMetadata(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/content/99/child/attachment" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{
					"id": "att1", "title": "diagram.png",
					"metadata": map[string]any{"mediaType": "image/png"},
					"_links":   map[string]any{"download": "/download/attachments/99/diagram.png?version=1"},
				},
			},
		})
	}))
	defer srv.Close()

	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	attachments, err := ListConfluenceAttachments(conn, "99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("len = %d, want 1", len(attachments))
	}
	got := attachments[0]
	if got.Title != "diagram.png" || got.MediaType != "image/png" {
		t.Errorf("unexpected attachment: %+v", got)
	}
	if got.DownloadPath != "/download/attachments/99/diagram.png?version=1" {
		t.Errorf("DownloadPath = %q", got.DownloadPath)
	}
}

func TestDownloadConfluenceAttachment_WritesFile(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/download/attachments/99/diagram.png" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte("fake-png-bytes"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "diagram.png")
	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	if err := DownloadConfluenceAttachment(conn, "/download/attachments/99/diagram.png", dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != "fake-png-bytes" {
		t.Errorf("content = %q, want fake-png-bytes", data)
	}
}

func TestDownloadConfluenceAttachment_NoDownloadPath(t *testing.T) {
	conn := JiraConnection{BaseURL: "https://example.com", Email: "u@example.com", APIToken: "tok"}
	if err := DownloadConfluenceAttachment(conn, "", "/tmp/x"); err == nil {
		t.Fatal("expected error for empty download path")
	}
}

func TestDownloadConfluenceAttachment_HTTPError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	err := DownloadConfluenceAttachment(conn, "/download/attachments/1/missing.png", filepath.Join(dir, "x.png"))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestResolveConfluenceDownloadURL(t *testing.T) {
	tests := []struct {
		name, base, path, want string
	}{
		{"relative path gets /wiki prepended", "https://org.atlassian.net", "/download/attachments/1/f.png", "https://org.atlassian.net/wiki/download/attachments/1/f.png"},
		{"already-prefixed path is used as-is", "https://org.atlassian.net", "/wiki/download/attachments/1/f.png", "https://org.atlassian.net/wiki/download/attachments/1/f.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveConfluenceDownloadURL(tt.base, tt.path); got != tt.want {
				t.Errorf("resolveConfluenceDownloadURL(%q, %q) = %q, want %q", tt.base, tt.path, got, tt.want)
			}
		})
	}
}

func TestDownloadConfluenceAttachmentsByFilename_DownloadsMatchesAndReportsMissing(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/child/attachment"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{
					map[string]any{
						"id": "att1", "title": "present.png",
						"_links": map[string]any{"download": "/download/attachments/99/present.png"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/present.png"):
			w.Write([]byte("bytes"))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	downloaded, missing, err := DownloadConfluenceAttachmentsByFilename(conn, "99", []string{"present.png", "missing.pdf"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(downloaded) != 1 || downloaded[0] != "present.png" {
		t.Errorf("downloaded = %v, want [present.png]", downloaded)
	}
	if len(missing) != 1 || missing[0] != "missing.pdf" {
		t.Errorf("missing = %v, want [missing.pdf]", missing)
	}
	if _, err := os.Stat(filepath.Join(dir, "present.png")); err != nil {
		t.Errorf("expected present.png to be written: %v", err)
	}
}

func TestDownloadConfluenceAttachmentsByFilename_Empty(t *testing.T) {
	conn := JiraConnection{BaseURL: "https://example.com", Email: "u@example.com", APIToken: "tok"}
	downloaded, missing, err := DownloadConfluenceAttachmentsByFilename(conn, "99", nil, t.TempDir())
	if err != nil || downloaded != nil || missing != nil {
		t.Errorf("expected no-op for empty wanted list, got downloaded=%v missing=%v err=%v", downloaded, missing, err)
	}
}

func TestDownloadConfluenceAttachmentsByFilename_SanitisesTraversal(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/child/attachment"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"results": []any{
					map[string]any{
						"id": "att1", "title": "../../etc/evil.png",
						"_links": map[string]any{"download": "/download/attachments/99/evil.png"},
					},
				},
			})
		default:
			w.Write([]byte("bytes"))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	conn := JiraConnection{BaseURL: srv.URL, Email: "u@example.com", APIToken: "tok"}
	downloaded, _, err := DownloadConfluenceAttachmentsByFilename(conn, "99", []string{"../../etc/evil.png"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(downloaded) != 1 {
		t.Fatalf("expected 1 download, got %v", downloaded)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.png")); err != nil {
		t.Errorf("expected sanitised filename evil.png under destDir: %v", err)
	}
}
