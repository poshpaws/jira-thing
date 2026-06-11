package version

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLatestRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://github.com/poshpaws/jira-thing/releases/tag/v1.2.3"}`))
	}))
	defer srv.Close()

	orig := HTTPClient
	HTTPClient = srv.Client()
	defer func() { HTTPClient = orig }()

	// Override the URL by patching the transport to redirect requests.
	HTTPClient.Transport = rewriteTransport{base: srv.URL}

	info, err := LatestRelease()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.TagName != "v1.2.3" {
		t.Errorf("got tag %q, want v1.2.3", info.TagName)
	}
}

func TestLatestRelease_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := HTTPClient
	HTTPClient = srv.Client()
	HTTPClient.Transport = rewriteTransport{base: srv.URL}
	defer func() { HTTPClient = orig }()

	_, err := LatestRelease()
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

func TestCheckMessage_UpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v1.0.0","html_url":"https://example.com"}`))
	}))
	defer srv.Close()

	orig := HTTPClient
	HTTPClient = srv.Client()
	HTTPClient.Transport = rewriteTransport{base: srv.URL}
	defer func() { HTTPClient = orig }()

	msg := CheckMessage("v1.0.0")
	if !strings.Contains(msg, "up to date") {
		t.Errorf("expected up to date message, got: %s", msg)
	}
}

func TestCheckMessage_UpdateAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v2.0.0","html_url":"https://example.com/release"}`))
	}))
	defer srv.Close()

	orig := HTTPClient
	HTTPClient = srv.Client()
	HTTPClient.Transport = rewriteTransport{base: srv.URL}
	defer func() { HTTPClient = orig }()

	msg := CheckMessage("v1.0.0")
	if !strings.Contains(msg, "update available") {
		t.Errorf("expected update available message, got: %s", msg)
	}
	if !strings.Contains(msg, "v2.0.0") {
		t.Errorf("expected new version in message, got: %s", msg)
	}
}

func TestCheckMessage_DevVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v3.0.0","html_url":"https://example.com"}`))
	}))
	defer srv.Close()

	orig := HTTPClient
	HTTPClient = srv.Client()
	HTTPClient.Transport = rewriteTransport{base: srv.URL}
	defer func() { HTTPClient = orig }()

	msg := CheckMessage("dev")
	if !strings.Contains(msg, "up to date") {
		t.Errorf("dev should be treated as up to date, got: %s", msg)
	}
}

// rewriteTransport redirects all requests to the test server.
type rewriteTransport struct {
	base string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.base, "http://")
	return http.DefaultTransport.RoundTrip(req)
}
