package ops

import (
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/auth"
	"jira-thing/internal/profile"
)

// maskedTokenEnds is how many characters of the token stay visible at each end.
const maskedTokenEnds = 3

// Diagnosis reports a connectivity check. TokenErr or APIErr is set at the
// first check that failed; checks after a failure are skipped. Hint, when
// set, suggests a fix for APIErr.
type Diagnosis struct {
	Profile     string // the active profile, "default" when none is selected
	Source      string // where the credentials came from
	URL         string
	Email       string
	MaskedToken string
	TokenErr    error
	APIErr      error
	Hint        string
	User        User
}

// OK reports whether every check passed.
func (d Diagnosis) OK() bool { return d.TokenErr == nil && d.APIErr == nil }

// Diagnose checks the credentials' token format, then calls the API as the
// current user.
func Diagnose(conn api.JiraConnection) Diagnosis {
	name, _ := profile.Active()
	d := Diagnosis{
		Profile: profile.Label(name), Source: auth.Source(),
		URL: conn.BaseURL, Email: conn.Email, MaskedToken: MaskToken(conn.APIToken),
	}
	if d.TokenErr = auth.ValidateToken(conn.APIToken); d.TokenErr != nil {
		return d
	}
	d.User, d.APIErr = WhoAmI(conn)
	if d.APIErr != nil {
		d.Hint = APIHint(d.APIErr)
	}
	return d
}

// APIHint suggests a fix for common authentication failures, or "" if none applies.
func APIHint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"):
		return "API token may be invalid or expired. Regenerate at https://id.atlassian.com/manage-profile/security/api-tokens"
	case strings.Contains(msg, "403"):
		return "Check that the email matches the token owner's Atlassian account"
	}
	return ""
}

// MaskToken shows only the first and last few characters of a token.
func MaskToken(token string) string {
	if len(token) <= 2*maskedTokenEnds {
		return "***"
	}
	return token[:maskedTokenEnds] + "..." + token[len(token)-maskedTokenEnds:]
}
