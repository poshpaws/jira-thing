package main

import (
	"strings"
	"testing"

	"jira-thing/internal/profile"
)

func TestApplyGlobalFlags(t *testing.T) {
	tests := []struct {
		args        []string
		wantArgs    string
		wantProfile string
		wantErr     string
	}{
		{[]string{"jt", "list", "-a", "me"}, "jt list -a me", "", ""},
		{[]string{"jt", "-profile", "work", "list"}, "jt list", "work", ""},
		{[]string{"jt", "--profile=client-a", "me"}, "jt me", "client-a", ""},
		{[]string{"jt", "-profile=x", "-profile", "y", "me"}, "jt me", "y", ""},
		{[]string{"jt", "list", "-profile", "work"}, "jt list -profile work", "", ""},
		{[]string{"jt", "-h"}, "jt -h", "", ""},
		{[]string{"jt", "-profile"}, "", "", "needs a name"},
		{[]string{"jt", "-profile="}, "", "", "needs a name"},
		{[]string{"jt", "-profile", "bad name", "list"}, "", "", "profile name"},
		{[]string{"jt"}, "jt", "", ""},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			t.Setenv(profile.EnvVar, "")
			got, err := applyGlobalFlags(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %v", err)
				}
				return
			}
			active, _ := profile.Active()
			if err != nil || strings.Join(got, " ") != tt.wantArgs || active != tt.wantProfile {
				t.Errorf("args=%q profile=%q err=%v", got, active, err)
			}
		})
	}
}
