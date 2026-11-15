package version

import (
	"context"
	"errors"
	"testing"
)

func TestSelfUpdate_RefusesHomebrewInstall(t *testing.T) {
	orig := executableFn
	defer func() { executableFn = orig }()
	executableFn = func() (string, error) { return "/opt/homebrew/Cellar/jira-thing/0.27.0/bin/jira-thing", nil }

	_, err := SelfUpdate(context.Background(), "v0.27.0")
	if !errors.Is(err, ErrHomebrewManaged) {
		t.Fatalf("expected ErrHomebrewManaged, got %v", err)
	}
}

func TestHomebrewManaged(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/Cellar/jira-thing/0.27.0/bin/jira-thing", true},
		{"/home/linuxbrew/.linuxbrew/Cellar/jira-thing/0.27.0/bin/jira-thing", true},
		{"/usr/local/bin/jira-thing", false},
		{"/Users/me/go/bin/jira-thing", false},
	}
	for _, tt := range tests {
		if got := homebrewManaged(tt.path); got != tt.want {
			t.Errorf("homebrewManaged(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
