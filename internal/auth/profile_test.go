package auth

import (
	"errors"
	"os"
	"strings"
	"testing"

	"jira-thing/internal/profile"
)

// TestMain clears the profile and credential variables, so a developer's
// own environment can't change which path the tests take.
func TestMain(m *testing.M) {
	for _, v := range []string{profile.EnvVar, EnvURL, EnvEmail, EnvToken} {
		_ = os.Unsetenv(v)
	}
	os.Exit(m.Run())
}

func TestGetCredentials_NamedProfileUsesItsOwnKeys(t *testing.T) {
	t.Setenv(profile.EnvVar, "work")
	kr := &mockKeyring{store: map[string]string{
		keyURL: "https://home.atlassian.net", keyEmail: "me@home", keyToken: "home-tok",
		keyURL + "@work": "https://work.atlassian.net", keyEmail + "@work": "me@work", keyToken + "@work": "work-tok",
	}}
	creds, err := getCredentials(kr)
	if err != nil || creds.URL != "https://work.atlassian.net" || creds.Token != "work-tok" {
		t.Errorf("creds=%+v err=%v", creds, err)
	}
}

func TestStoreAndClear_OnlyTouchActiveProfile(t *testing.T) {
	t.Setenv(profile.EnvVar, "work")
	kr := &mockKeyring{store: map[string]string{keyURL: "https://home", keyEmail: "h", keyToken: "h"}}
	if err := storeCredentials(kr, Credentials{URL: "https://work", Email: "w", Token: "w"}); err != nil {
		t.Fatal(err)
	}
	if kr.store[keyURL+"@work"] != "https://work" || kr.store[keyURL] != "https://home" {
		t.Errorf("store = %v", kr.store)
	}
	if err := clearCredentials(kr); err != nil {
		t.Fatal(err)
	}
	if _, ok := kr.store[keyURL+"@work"]; ok || kr.store[keyURL] != "https://home" {
		t.Errorf("after clear = %v", kr.store)
	}
}

func TestGetCredentials_BadProfileName(t *testing.T) {
	t.Setenv(profile.EnvVar, "bad name")
	if _, err := getCredentials(&mockKeyring{store: map[string]string{}}); err == nil || !strings.Contains(err.Error(), "profile name") {
		t.Errorf("err = %v", err)
	}
}

func TestGetCredentials_EnvOverride(t *testing.T) {
	t.Setenv(EnvURL, "https://ci.atlassian.net/")
	t.Setenv(EnvEmail, "ci@x.io")
	t.Setenv(EnvToken, "ci-token")
	kr := &mockKeyring{store: map[string]string{keyURL: "https://home", keyEmail: "h", keyToken: "h"}}
	creds, err := getCredentials(kr)
	if err != nil || creds != (Credentials{URL: "https://ci.atlassian.net", Email: "ci@x.io", Token: "ci-token"}) {
		t.Errorf("creds=%+v err=%v", creds, err)
	}
	if !strings.HasPrefix(Source(), "environment") {
		t.Errorf("Source = %q", Source())
	}
}

func TestGetCredentials_PartialEnvIsAnError(t *testing.T) {
	t.Setenv(EnvToken, "only-token")
	kr := &mockKeyring{store: map[string]string{keyURL: "https://home", keyEmail: "h", keyToken: "h"}}
	if _, err := getCredentials(kr); !errors.Is(err, ErrPartialEnv) {
		t.Errorf("err = %v", err)
	}
	if Source() != "keyring" {
		t.Errorf("Source = %q", Source())
	}
}

func TestGetCredentials_EnvTokenValidated(t *testing.T) {
	t.Setenv(EnvURL, "https://ci")
	t.Setenv(EnvEmail, "ci@x.io")
	t.Setenv(EnvToken, "has space")
	if _, err := getCredentials(&mockKeyring{store: map[string]string{}}); err == nil || !strings.Contains(err.Error(), "whitespace") {
		t.Errorf("err = %v", err)
	}
}
