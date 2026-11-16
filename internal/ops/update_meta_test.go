package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
)

// metaJira serves an edit screen with Bug/Task types and the given extra
// fields, recording PUT bodies.
type metaJira struct {
	extra   []string
	puts    []map[string]any
	metaHit int
}

func (j *metaJira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if strings.HasSuffix(r.URL.Path, "/editmeta") {
		j.metaHit++
		fields := map[string]any{"issuetype": map[string]any{"name": "Issue Type", "allowedValues": []map[string]any{
			{"id": "10001", "name": "Bug"}, {"id": "10003", "name": "Task"},
		}}}
		for _, f := range j.extra {
			fields[f] = map[string]any{"name": f}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"fields": fields})
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	j.puts = append(j.puts, body)
	w.WriteHeader(http.StatusNoContent)
}

func startMeta(t *testing.T, j *metaJira) api.JiraConnection {
	t.Helper()
	srv := httptest.NewTLSServer(j)
	t.Cleanup(srv.Close)
	return api.JiraConnection{BaseURL: srv.URL, Email: "a@b.c", APIToken: "tok"}
}

func points(v float64) *float64 { return &v }

func TestUpdateTicket_TypeAndPoints(t *testing.T) {
	j := &metaJira{extra: []string{"customfield_10308"}}
	res, err := UpdateTicket(startMeta(t, j), UpdateRequest{Key: "K-1", IssueType: "task", StoryPoints: points(0.5)}, false)
	if err != nil || res.IssueType != "Task" || res.PointsField != "customfield_10308" || j.metaHit != 1 {
		t.Fatalf("res=%+v metaHit=%d err=%v", res, j.metaHit, err)
	}
	fields, _ := j.puts[0]["fields"].(map[string]any)
	if nestedStr(fields, "issuetype", "id") != "10003" || fields["customfield_10308"] != 0.5 {
		t.Errorf("PUT fields = %v", fields)
	}
}

func TestUpdateTicket_ConfiguredPointsFieldFirst(t *testing.T) {
	j := &metaJira{extra: []string{"customfield_10016", "customfield_12345"}}
	pf := StoryPointFields(config.Config{StoryPointsField: "customfield_12345"})
	res, err := UpdateTicket(startMeta(t, j), UpdateRequest{Key: "K-1", StoryPoints: points(3), PointFields: pf}, true)
	if err != nil || res.PointsField != "customfield_12345" || res.Status != StatusDryRun || len(j.puts) != 0 {
		t.Errorf("res=%+v puts=%v err=%v", res, j.puts, err)
	}
}

func TestUpdateTicket_MetaErrors(t *testing.T) {
	tests := []struct {
		name  string
		req   UpdateRequest
		check func(error) bool
	}{
		{"unknown type", UpdateRequest{Key: "K-1", IssueType: "Epic"}, func(err error) bool {
			var u *UnknownIssueTypeError
			return errors.As(err, &u) && strings.Contains(err.Error(), "Bug, Task") && strings.Contains(err.Error(), "Move")
		}},
		{"no points field", UpdateRequest{Key: "K-1", StoryPoints: points(1)}, func(err error) bool { return errors.Is(err, ErrNoPointsField) }},
		{"negative points", UpdateRequest{Key: "K-1", StoryPoints: points(-1)}, func(err error) bool { return errors.Is(err, ErrBadStoryPoints) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &metaJira{}
			_, err := UpdateTicket(startMeta(t, j), tt.req, false)
			if !tt.check(err) || len(j.puts) != 0 {
				t.Errorf("err=%v puts=%v", err, j.puts)
			}
		})
	}
}

func TestUnknownIssueTypeError_OneLineNames(t *testing.T) {
	meta := map[string]api.EditField{"issuetype": {AllowedValues: []map[string]any{{"id": "1", "name": "Bug\nForged: yes"}}}}
	err := setIssueType(meta, UpdateRequest{Key: "K-1", IssueType: "x\ny"}, &UpdateResult{Fields: map[string]any{}})
	if strings.Contains(err.Error(), "Bug\n") || strings.Contains(err.Error(), "x\ny") || !strings.Contains(err.Error(), "Bug Forged: yes") {
		t.Errorf("err = %q", err)
	}
}

func TestUpdateTicket_NoMetaFetchWithoutTypeOrPoints(t *testing.T) {
	j := &metaJira{}
	if _, err := UpdateTicket(startMeta(t, j), UpdateRequest{Key: "K-1", Summary: "S"}, false); err != nil || j.metaHit != 0 {
		t.Errorf("metaHit=%d err=%v", j.metaHit, err)
	}
}

func TestStoryPointFieldsAndValue(t *testing.T) {
	if got := StoryPointFields(config.Config{}); strings.Join(got, ",") != strings.Join(DefaultStoryPointFields, ",") {
		t.Errorf("default = %v", got)
	}
	if got := StoryPointFields(config.Config{StoryPointsField: "customfield_10016"}); len(got) != len(DefaultStoryPointFields) {
		t.Errorf("configured default duplicated: %v", got)
	}
	if v, ok := StoryPoints(map[string]any{"customfield_10016": nil, "customfield_10308": 2.0}, DefaultStoryPointFields); !ok || v != 2.0 {
		t.Errorf("v=%v ok=%v", v, ok)
	}
	if _, ok := StoryPoints(nil, DefaultStoryPointFields); ok {
		t.Error("nil fields should have no points")
	}
}
