package auth

import (
	"errors"
	"testing"
)

// mockKeyring is an in-memory Keyring for tests.
type mockKeyring struct {
	store map[string]string
	delErr error
}

func (m *mockKeyring) Get(service, key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}
func (m *mockKeyring) Set(service, key, value string) error {
	m.store[key] = value
	return nil
}
func (m *mockKeyring) Delete(service, key string) error {
	if m.delErr != nil {
		return m.delErr
	}
	delete(m.store, key)
	return nil
}

func TestGetCredentials_ReturnsStored(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{
		keyURL:   "https://x.atlassian.net",
		keyEmail: "a@b.com",
		keyToken: "tok",
	}}
	url, email, token, err := getCredentials(kr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://x.atlassian.net" {
		t.Errorf("url = %q, want %q", url, "https://x.atlassian.net")
	}
	if email != "a@b.com" {
		t.Errorf("email = %q, want %q", email, "a@b.com")
	}
	if token != "tok" {
		t.Errorf("token = %q, want %q", token, "tok")
	}
}

func TestGetCredentials_MissingTriggersPrompt(t *testing.T) {
	// With empty store, getCredentials calls promptAndStore which reads from
	// stdin — we can't easily test that path in a unit test without a PTY.
	// This test verifies that missing credentials cause the prompt path to be
	// taken by checking that the returned error is non-nil when the store is
	// empty (promptAndStore fails because stdin is not a terminal in tests).
	kr := &mockKeyring{store: map[string]string{}}
	_, _, _, err := getCredentials(kr)
	if err == nil {
		t.Fatal("expected error when credentials missing and stdin is not a TTY")
	}
}

func TestClearCredentials_DeletesAllKeys(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{
		keyURL:   "u",
		keyEmail: "e",
		keyToken: "t",
	}}
	clearCredentials(kr)
	if len(kr.store) != 0 {
		t.Errorf("expected empty store after clear, got %v", kr.store)
	}
}

func TestClearCredentials_IgnoresDeleteErrors(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{}, delErr: errors.New("not found")}
	clearCredentials(kr) // must not panic or return error
}
