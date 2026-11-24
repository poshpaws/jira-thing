package main

import (
	"errors"
	"slices"
	"testing"

	"jira-thing/internal/ops"
)

func TestMenuCreateArgs(t *testing.T) {
	tests := []struct {
		name, sprint, points string
		want                 []string
		wantErr              error
	}{
		{"nothing", "", "", nil, nil},
		{"sprint and points", "current", "0.5", []string{"-sprint", "current", "-points", "0.5"}, nil},
		{"points only", "", "3", []string{"-points", "3"}, nil},
		{"closed sprint", "prev", "", nil, ops.ErrClosedSprintTarget},
		{"unknown sprint", "someday", "", nil, ops.ErrBadSprint},
		{"bad points", "", "lots", nil, ops.ErrBadStoryPoints},
		{"negative points", "", "-1", nil, ops.ErrBadStoryPoints},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := menuCreateArgs(tt.sprint, tt.points)
			if !errors.Is(err, tt.wantErr) || !slices.Equal(got, tt.want) {
				t.Errorf("got %v err=%v", got, err)
			}
		})
	}
}
