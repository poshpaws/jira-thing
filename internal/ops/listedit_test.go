package ops

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseListEdit(t *testing.T) {
	tests := []struct {
		in      string
		want    ListEdit
		wantErr bool
	}{
		{"backend", ListEdit{Value: "backend"}, false},
		{"+backend", ListEdit{Value: "backend"}, false},
		{"-old", ListEdit{Remove: true, Value: "old"}, false},
		{" - spaced ", ListEdit{Remove: true, Value: "spaced"}, false},
		{"-", ListEdit{}, true},
		{"+", ListEdit{}, true},
		{"", ListEdit{}, true},
	}
	for _, tt := range tests {
		got, err := ParseListEdit(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseListEdit(%q) = %+v, %v", tt.in, got, err)
		}
	}
}

func TestUpdateTicket_ListEdits(t *testing.T) {
	j := &metaJira{}
	edits := ListEdits{
		Labels:      []ListEdit{{Value: "new"}, {Remove: true, Value: "old"}},
		Components:  []ListEdit{{Value: "api"}},
		FixVersions: []ListEdit{{Remove: true, Value: "v1"}},
	}
	res, err := UpdateTicket(startMeta(t, j), UpdateRequest{Key: "K-1", Summary: "S", ListEdits: edits}, false)
	if err != nil || len(j.puts) != 1 {
		t.Fatalf("res=%+v puts=%v err=%v", res, j.puts, err)
	}
	body, _ := json.Marshal(j.puts[0])
	want := `{"fields":{"summary":"S"},"update":{"components":[{"add":{"name":"api"}}],"fixVersions":[{"remove":{"name":"v1"}}],"labels":[{"add":"new"},{"remove":"old"}]}}`
	if string(body) != want {
		t.Errorf("body = %s", body)
	}
}

func TestUpdateTicket_ListEditsOnly(t *testing.T) {
	j := &metaJira{}
	res, err := UpdateTicket(startMeta(t, j), UpdateRequest{Key: "K-1", ListEdits: ListEdits{Labels: []ListEdit{{Value: "x"}}}}, true)
	if err != nil || res.Status != StatusDryRun || len(res.Fields) != 0 || res.Update["labels"] == nil || len(j.puts) != 0 {
		t.Errorf("res=%+v puts=%v err=%v", res, j.puts, err)
	}
}

func TestUpdateTicket_ReplaceAndEditConflict(t *testing.T) {
	_, err := UpdateTicket(startMeta(t, &metaJira{}), UpdateRequest{
		Key: "K-1", Labels: []string{"a"}, ListEdits: ListEdits{Labels: []ListEdit{{Value: "b"}}},
	}, false)
	if err == nil || !strings.Contains(err.Error(), "labels can't be replaced and edited at once") {
		t.Errorf("err = %v", err)
	}
	if _, err := UpdateTicket(startMeta(t, &metaJira{}), UpdateRequest{Key: "K-1"}, false); !errors.Is(err, ErrNothingToUpdate) {
		t.Errorf("empty: err = %v", err)
	}
}
