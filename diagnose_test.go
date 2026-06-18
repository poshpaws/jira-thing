package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunDiagnose_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"accountId":   "123abc",
			"displayName": "Test User",
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runDiagnose(nil) })
	if !strings.Contains(out, "Credentials loaded") {
		t.Errorf("expected credentials loaded message, got: %s", out)
	}
	if !strings.Contains(out, "API connection successful") {
		t.Errorf("expected API success message, got: %s", out)
	}
	if !strings.Contains(out, "Test User") {
		t.Errorf("expected display name, got: %s", out)
	}
}

func TestRunDiagnose_CredentialError(t *testing.T) {
	defer mockCredsError(fmt.Errorf("no keyring available"))()

	out := captureStdout(func() { runDiagnose(nil) })
	if !strings.Contains(out, "Credentials") {
		t.Errorf("expected credential error, got: %s", out)
	}
	if !strings.Contains(out, "no keyring available") {
		t.Errorf("expected error detail, got: %s", out)
	}
}

func TestRunDiagnose_APIError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runDiagnose(nil) })
	if !strings.Contains(out, "API connection") {
		t.Errorf("expected API error message, got: %s", out)
	}
	if !strings.Contains(out, "401") {
		t.Errorf("expected 401 in output, got: %s", out)
	}
	if !strings.Contains(out, "invalid or expired") {
		t.Errorf("expected helpful hint, got: %s", out)
	}
}

func TestRunDiagnose_UserID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"accountId":   "712020:abc123def",
			"displayName": "Test User",
		})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	out := captureStdout(func() { runDiagnose([]string{"-userid"}) })
	if strings.TrimSpace(out) != "712020:abc123def" {
		t.Errorf("expected accountId only, got: %q", out)
	}
}

func TestFormatDiagError_401(t *testing.T) {
	msg := formatDiagError(fmt.Errorf("HTTP 401: Unauthorized"))
	if !strings.Contains(msg, "invalid or expired") {
		t.Errorf("expected token hint, got: %s", msg)
	}
}

func TestFormatDiagError_403(t *testing.T) {
	msg := formatDiagError(fmt.Errorf("HTTP 403: Forbidden"))
	if !strings.Contains(msg, "email matches") {
		t.Errorf("expected email hint, got: %s", msg)
	}
}

func TestFormatDiagError_Other(t *testing.T) {
	msg := formatDiagError(fmt.Errorf("connection refused"))
	if msg != "connection refused" {
		t.Errorf("expected unchanged message, got: %s", msg)
	}
}
