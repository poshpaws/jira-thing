package ops

import (
	"errors"
	"strings"
	"testing"

	"jira-thing/internal/api"
	"jira-thing/internal/auth"
	"jira-thing/internal/profile"
)

func TestDiagnose_AllChecksPass(t *testing.T) {
	conn := startLifecycle(t, &lifecycleJira{})
	d := Diagnose(conn)
	if !d.OK() || d.User.AccountID != "acc-me" || d.MaskedToken != "***" {
		t.Errorf("d=%+v", d)
	}
}

func TestDiagnose_BadTokenStopsBeforeAPI(t *testing.T) {
	d := Diagnose(api.JiraConnection{BaseURL: "https://unused.invalid", APIToken: "has space"})
	if d.TokenErr == nil || d.APIErr != nil || d.OK() {
		t.Errorf("d=%+v", d)
	}
}

func TestAPIHint(t *testing.T) {
	tests := []struct{ err, want string }{
		{"HTTP 401: Unauthorized", "invalid or expired"},
		{"HTTP 403: Forbidden", "email matches"},
		{"connection refused", ""},
	}
	for _, tt := range tests {
		got := APIHint(errors.New(tt.err))
		if (tt.want == "") != (got == "") || (tt.want != "" && !strings.Contains(got, tt.want)) {
			t.Errorf("APIHint(%q) = %q", tt.err, got)
		}
	}
}

func TestMaskToken(t *testing.T) {
	if got := MaskToken("abcdefghij"); got != "abc...hij" {
		t.Errorf("got %q", got)
	}
}

func TestDiagnose_ReportsProfileAndSource(t *testing.T) {
	t.Setenv(profile.EnvVar, "work")
	for _, v := range []string{auth.EnvURL, auth.EnvEmail, auth.EnvToken} {
		t.Setenv(v, "")
	}
	d := Diagnose(api.JiraConnection{BaseURL: "https://x", APIToken: "has space"})
	if d.Profile != "work" || d.Source != "keyring" {
		t.Errorf("profile=%q source=%q", d.Profile, d.Source)
	}
}
