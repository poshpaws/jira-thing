package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// putRecorder serves 204 to PUTs and POSTs, recording each body.
func putRecorder(t *testing.T) *[]map[string]any {
	t.Helper()
	bodies := &[]map[string]any{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		*bodies = append(*bodies, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(mockCreds(srv.URL))
	return bodies
}

func TestRunEdit_DescriptionFromStdin(t *testing.T) {
	bodies := putRecorder(t)
	defer pipeStdinLines(t, "## Plan", "", "- **ship** it")()
	captureStdout(func() { runEdit([]string{"PROJ-1", "-description-file", "-"}) })
	body, _ := json.Marshal((*bodies)[0])
	if !strings.Contains(string(body), `"type":"heading"`) || !strings.Contains(string(body), `"text":"ship"`) {
		t.Errorf("body = %s", body)
	}
}

func TestRunCreate_DescriptionFromStdinIsNonInteractive(t *testing.T) {
	putRecorder(t)
	defer pipeStdinLines(t, "piped body")()
	out := captureStdout(func() {
		runCreate([]string{"-project", "P", "-summary", "From a pipe", "-description-file", "-", "-dry-run"})
	})
	if !strings.Contains(out, `"text": "piped body"`) {
		t.Errorf("out = %s", out)
	}
}

func TestRunCreate_StdinWithoutSummaryFails(t *testing.T) {
	putRecorder(t)
	defer pipeStdinLines(t, "body only")()
	errOut := captureStderr(func() {
		if !captureExit(func() { runCreate([]string{"-project", "P", "-description-file", "-", "-dry-run"}) }) {
			t.Error("expected exit without a summary")
		}
	})
	if !strings.Contains(strings.ToLower(errOut), "summary") {
		t.Errorf("stderr = %s", errOut)
	}
}

func TestReadDescriptionFile_EmptyStdin(t *testing.T) {
	defer pipeStdinLines(t, "  ")()
	if _, err := readDescriptionFile("-"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("err = %v", err)
	}
}

func TestRunCreate_FixVersions(t *testing.T) {
	putRecorder(t)
	out := captureStdout(func() {
		runCreate([]string{"-y", "-project", "P", "-summary", "S", "-fix-version", "v1", "-fix-version", "v2,v3", "-dry-run"})
	})
	for _, v := range []string{`"name": "v1"`, `"name": "v2"`, `"name": "v3"`} {
		if !strings.Contains(out, v) {
			t.Errorf("missing %s in %s", v, out)
		}
	}
}
