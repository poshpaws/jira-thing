package auth

import (
	"errors"
	"os"
	"strings"
)

// Environment variables that together override stored credentials, for CI
// and other places without a keyring.
const (
	EnvURL   = "JIRA_URL"
	EnvEmail = "JIRA_EMAIL"
	EnvToken = "JIRA_API_TOKEN" // #nosec G101 -- variable name, not a credential value
)

// ErrPartialEnv means some but not all of the credential variables are set.
var ErrPartialEnv = errors.New("set all of JIRA_URL, JIRA_EMAIL and JIRA_API_TOKEN, or none of them")

// envCredentials returns credentials from the environment. ok is false when
// none of the variables is set; setting only some is an error, so a stored
// URL is never paired with another account's token.
func envCredentials() (Credentials, bool, error) {
	creds := Credentials{
		URL:   strings.TrimRight(strings.TrimSpace(os.Getenv(EnvURL)), "/"),
		Email: strings.TrimSpace(os.Getenv(EnvEmail)),
		Token: strings.TrimSpace(os.Getenv(EnvToken)),
	}
	set := 0
	for _, v := range []string{creds.URL, creds.Email, creds.Token} {
		if v != "" {
			set++
		}
	}
	switch set {
	case 0:
		return Credentials{}, false, nil
	case 3:
		return creds, true, ValidateToken(creds.Token)
	}
	return Credentials{}, false, ErrPartialEnv
}

// Source describes where credentials come from: the environment, or the
// keyring under the active profile.
func Source() string {
	if _, ok, _ := envCredentials(); ok {
		return "environment (JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN)"
	}
	return "keyring"
}
