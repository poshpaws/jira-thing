package main

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAttach_MultipleFilesViaFFlag(t *testing.T) {
	var uploaded []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/attachments") {
			t.Errorf("expected /attachments path, got %s", r.URL.Path)
		}
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parsing content type: %v", err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("reading multipart: %v", err)
		}
		uploaded = append(uploaded, part.FileName())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{map[string]any{"id": "1", "filename": part.FileName()}})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()

	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")
	os.WriteFile(path1, []byte("a"), 0o600)
	os.WriteFile(path2, []byte("b"), 0o600)

	out := captureStdout(func() { runAttach([]string{"PROJ-1", "-f", path1, "-f", path2}) })
	if len(uploaded) != 2 {
		t.Fatalf("expected 2 uploads, got %d: %v", len(uploaded), uploaded)
	}
	if uploaded[0] != "a.txt" || uploaded[1] != "b.txt" {
		t.Errorf("expected a.txt then b.txt, got %v", uploaded)
	}
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("expected ticket key in output, got: %s", out)
	}
}

func TestRunAttach_StdinWithName(t *testing.T) {
	var uploadedName string
	var uploadedBody string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parsing content type: %v", err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("reading multipart: %v", err)
		}
		uploadedName = part.FileName()
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading part body: %v", err)
		}
		uploadedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{map[string]any{"id": "1", "filename": uploadedName}})
	}))
	defer srv.Close()
	defer mockCreds(srv.URL)()
	defer pipeStdinLines(t, "hello from stdin")()

	out := captureStdout(func() { runAttach([]string{"PROJ-1", "--stdin", "--name", "note.md"}) })
	if uploadedName != "note.md" {
		t.Errorf("expected filename note.md, got %q", uploadedName)
	}
	if !strings.Contains(uploadedBody, "hello from stdin") {
		t.Errorf("expected stdin content uploaded, got %q", uploadedBody)
	}
	if !strings.Contains(out, "PROJ-1") {
		t.Errorf("expected ticket key in output, got: %s", out)
	}
}

func TestRunAttach_StdinWithoutNameExits(t *testing.T) {
	defer mockCreds("https://example.invalid")()
	defer pipeStdinLines(t, "content")()

	exited := captureExit(func() {
		captureStderr(func() { runAttach([]string{"PROJ-1", "--stdin"}) })
	})
	if !exited {
		t.Error("expected osExit when -stdin is given without -name")
	}
}

func TestRunAttach_NoFilesExits(t *testing.T) {
	defer mockCreds("https://example.invalid")()

	exited := captureExit(func() {
		captureStderr(func() { runAttach([]string{"PROJ-1"}) })
	})
	if !exited {
		t.Error("expected osExit when no file, -f, or -stdin is given")
	}
}
