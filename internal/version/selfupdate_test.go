package version

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withStubbedRelease(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	origClient := HTTPClient
	HTTPClient = srv.Client()
	HTTPClient.Transport = rewriteTransport{base: srv.URL}
	t.Cleanup(func() { HTTPClient = origClient })
}

func TestSelfUpdate_DevBuildRejected(t *testing.T) {
	_, err := SelfUpdate(context.Background(), "dev")
	if err == nil || !strings.Contains(err.Error(), "dev build") {
		t.Fatalf("expected dev build error, got: %v", err)
	}
}

func TestSelfUpdate_AlreadyUpToDate(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v1.0.0","html_url":"https://example.com"}`)

	msg, err := SelfUpdate(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "up to date") {
		t.Errorf("expected up to date message, got: %s", msg)
	}
}

func TestSelfUpdate_NoMatchingAsset(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v2.0.0","html_url":"https://example.com","assets":[{"name":"jira-thing-plan9-amd64","browser_download_url":"https://example.com/asset"}]}`)

	_, err := SelfUpdate(context.Background(), "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "no release asset found") {
		t.Fatalf("expected no-asset error, got: %v", err)
	}
}

func TestSelfUpdate_DownloadError(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v2.0.0","html_url":"https://example.com","assets":[{"name":"jira-thing-`+assetSuffix()+`","browser_download_url":"https://example.com/asset"}]}`)

	origDownload := downloadFn
	defer func() { downloadFn = origDownload }()
	downloadFn = func(ctx context.Context, url string) (io.ReadCloser, error) {
		return nil, errors.New("network down")
	}

	_, err := SelfUpdate(context.Background(), "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("expected wrapped download error, got: %v", err)
	}
}

func TestSelfUpdate_Success(t *testing.T) {
	withStubbedRelease(t, `{"tag_name":"v2.0.0","html_url":"https://example.com","assets":[{"name":"jira-thing-`+assetSuffix()+`","browser_download_url":"https://example.com/asset"}]}`)

	origDownload := downloadFn
	defer func() { downloadFn = origDownload }()
	downloadFn = func(ctx context.Context, url string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("fake binary contents")), nil
	}

	origApply := applyFn
	defer func() { applyFn = origApply }()
	var applied string
	applyFn = func(r io.Reader) error {
		data, err := io.ReadAll(r)
		applied = string(data)
		return err
	}

	msg, err := SelfUpdate(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applied != "fake binary contents" {
		t.Errorf("expected apply to receive downloaded bytes, got: %q", applied)
	}
	if !strings.Contains(msg, "v1.0.0 → v2.0.0") {
		t.Errorf("expected version transition in message, got: %s", msg)
	}
}
