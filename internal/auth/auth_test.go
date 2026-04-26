package auth

import (
	"errors"
	"fmt"
	"os"
	"testing"

	gokeyring "github.com/zalando/go-keyring"
)

// mockKeyring is an in-memory Keyring for tests.
type mockKeyring struct {
	store  map[string]string
	delErr error
}

func (m *mockKeyring) Get(key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", gokeyring.ErrNotFound
	}
	return v, nil
}
func (m *mockKeyring) Set(key, value string) error {
	m.store[key] = value
	return nil
}
func (m *mockKeyring) Delete(key string) error {
	if m.delErr != nil {
		return m.delErr
	}
	delete(m.store, key)
	return nil
}

// errGetKeyring returns a configured error for a specific key on Get.
type errGetKeyring struct {
	store  map[string]string
	errKey string
	getErr error
}

func (e *errGetKeyring) Get(key string) (string, error) {
	if key == e.errKey {
		return "", e.getErr
	}
	v, ok := e.store[key]
	if !ok {
		return "", gokeyring.ErrNotFound
	}
	return v, nil
}
func (e *errGetKeyring) Set(key, value string) error { e.store[key] = value; return nil }
func (e *errGetKeyring) Delete(key string) error     { delete(e.store, key); return nil }

// setErrKeyring returns a configured error for a specific key on Set.
type setErrKeyring struct {
	store    map[string]string
	setOnKey string
	setErr   error
}

func (s *setErrKeyring) Get(key string) (string, error) {
	v, ok := s.store[key]
	if !ok {
		return "", gokeyring.ErrNotFound
	}
	return v, nil
}
func (s *setErrKeyring) Set(key, value string) error {
	if s.setOnKey == "" || key == s.setOnKey {
		return s.setErr
	}
	s.store[key] = value
	return nil
}
func (s *setErrKeyring) Delete(key string) error { delete(s.store, key); return nil }

// --- getCredentials ---

func TestGetCredentials_ReturnsStored(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{
		keyURL:   "https://x.atlassian.net",
		keyEmail: "a@b.com",
		keyToken: "tok",
	}}
	creds, err := getCredentials(kr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.URL != "https://x.atlassian.net" {
		t.Errorf("URL = %q, want %q", creds.URL, "https://x.atlassian.net")
	}
	if creds.Email != "a@b.com" {
		t.Errorf("Email = %q, want %q", creds.Email, "a@b.com")
	}
	if creds.Token != "tok" {
		t.Errorf("Token = %q, want %q", creds.Token, "tok")
	}
}

func TestGetCredentials_MissingTriggersPrompt(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{}}
	_, err := getCredentials(kr)
	if err == nil {
		t.Fatal("expected error when credentials missing and stdin is not a TTY")
	}
}

func TestGetCredentials_URLKeyringError(t *testing.T) {
	kr := &errGetKeyring{store: map[string]string{}, errKey: keyURL, getErr: errors.New("keyring unavailable")}
	_, err := getCredentials(kr)
	if err == nil {
		t.Fatal("expected error for URL keyring failure")
	}
}

func TestGetCredentials_EmailKeyringError(t *testing.T) {
	kr := &errGetKeyring{
		store:  map[string]string{keyURL: "https://x.atlassian.net"},
		errKey: keyEmail,
		getErr: errors.New("keyring unavailable"),
	}
	_, err := getCredentials(kr)
	if err == nil {
		t.Fatal("expected error for email keyring failure")
	}
}

func TestGetCredentials_TokenKeyringError(t *testing.T) {
	kr := &errGetKeyring{
		store:  map[string]string{keyURL: "https://x.atlassian.net", keyEmail: "a@b.com"},
		errKey: keyToken,
		getErr: errors.New("keyring unavailable"),
	}
	_, err := getCredentials(kr)
	if err == nil {
		t.Fatal("expected error for token keyring failure")
	}
}

// --- storeCredentials ---

func TestStoreCredentials_Success(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{}}
	creds := Credentials{URL: "https://x.com", Email: "a@b.com", Token: "tok"}
	if err := storeCredentials(kr, creds); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kr.store[keyURL] != "https://x.com" {
		t.Error("URL not stored")
	}
	if kr.store[keyEmail] != "a@b.com" {
		t.Error("email not stored")
	}
	if kr.store[keyToken] != "tok" {
		t.Error("token not stored")
	}
}

func TestStoreCredentials_URLSetError(t *testing.T) {
	kr := &setErrKeyring{store: map[string]string{}, setOnKey: keyURL, setErr: errors.New("set failed")}
	err := storeCredentials(kr, Credentials{URL: "u", Email: "e", Token: "t"})
	if err == nil {
		t.Fatal("expected error for URL set failure")
	}
}

func TestStoreCredentials_EmailSetError(t *testing.T) {
	kr := &setErrKeyring{store: map[string]string{}, setOnKey: keyEmail, setErr: errors.New("set failed")}
	err := storeCredentials(kr, Credentials{URL: "u", Email: "e", Token: "t"})
	if err == nil {
		t.Fatal("expected error for email set failure")
	}
}

func TestStoreCredentials_TokenSetError(t *testing.T) {
	kr := &setErrKeyring{store: map[string]string{}, setOnKey: keyToken, setErr: errors.New("set failed")}
	err := storeCredentials(kr, Credentials{URL: "u", Email: "e", Token: "t"})
	if err == nil {
		t.Fatal("expected error for token set failure")
	}
}

// --- clearCredentials ---

func TestClearCredentials_DeletesAllKeys(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{
		keyURL:   "u",
		keyEmail: "e",
		keyToken: "t",
	}}
	if err := clearCredentials(kr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kr.store) != 0 {
		t.Errorf("expected empty store after clear, got %v", kr.store)
	}
}

func TestClearCredentials_IgnoresNotFoundErrors(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{}, delErr: gokeyring.ErrNotFound}
	if err := clearCredentials(kr); err != nil {
		t.Errorf("expected no error for not-found deletes, got %v", err)
	}
}

func TestClearCredentials_DeleteError(t *testing.T) {
	kr := &mockKeyring{store: map[string]string{keyURL: "u"}, delErr: errors.New("delete failed")}
	err := clearCredentials(kr)
	if err == nil {
		t.Fatal("expected error for non-NotFound delete failure")
	}
}

// --- public wrappers ---

func TestGetCredentials_UsesBackend(t *testing.T) {
	old := backend
	defer func() { backend = old }()
	backend = &mockKeyring{store: map[string]string{
		keyURL:   "https://x.atlassian.net",
		keyEmail: "a@b.com",
		keyToken: "tok",
	}}
	creds, err := GetCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.URL != "https://x.atlassian.net" {
		t.Errorf("URL = %q", creds.URL)
	}
}

func TestClearCredentials_UsesBackend(t *testing.T) {
	old := backend
	defer func() { backend = old }()
	backend = &mockKeyring{store: map[string]string{
		keyURL:   "u",
		keyEmail: "e",
		keyToken: "t",
	}}
	if err := ClearCredentials(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- readCredentials ---

func pipeStdin(t *testing.T, lines ...string) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	return func() {
		os.Stdin = old
		r.Close()
	}
}

func mockReadPassword(t *testing.T, token string, err error) func() {
	t.Helper()
	old := readPassword
	readPassword = func() ([]byte, error) { return []byte(token), err }
	return func() { readPassword = old }
}

func TestReadCredentials_Success(t *testing.T) {
	defer pipeStdin(t, "https://example.atlassian.net", "user@example.com")()
	defer mockReadPassword(t, "mytoken", nil)()

	creds, err := readCredentials()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.URL != "https://example.atlassian.net" {
		t.Errorf("URL = %q", creds.URL)
	}
	if creds.Email != "user@example.com" {
		t.Errorf("email = %q", creds.Email)
	}
	if creds.Token != "mytoken" {
		t.Errorf("token = %q", creds.Token)
	}
}

func TestReadCredentials_URLReadError(t *testing.T) {
	// Close pipe immediately so ReadString returns EOF.
	r, w, _ := os.Pipe()
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old; r.Close() }()

	_, err := readCredentials()
	if err == nil {
		t.Fatal("expected error for URL read failure")
	}
}

func TestReadCredentials_EmailReadError(t *testing.T) {
	// Write URL only, then close — email read hits EOF.
	defer pipeStdin(t, "https://example.atlassian.net")()

	_, err := readCredentials()
	if err == nil {
		t.Fatal("expected error for email read failure")
	}
}

func TestReadCredentials_PasswordReadError(t *testing.T) {
	defer pipeStdin(t, "https://example.atlassian.net", "user@example.com")()
	defer mockReadPassword(t, "", errors.New("terminal error"))()

	_, err := readCredentials()
	if err == nil {
		t.Fatal("expected error for password read failure")
	}
}

func TestReadCredentials_EmptyFields(t *testing.T) {
	defer pipeStdin(t, "", "user@example.com")()
	defer mockReadPassword(t, "", nil)()

	_, err := readCredentials()
	if err == nil {
		t.Fatal("expected error for empty fields")
	}
}

// --- promptAndStore ---

func TestPromptAndStore_Success(t *testing.T) {
	defer pipeStdin(t, "https://example.atlassian.net", "user@example.com")()
	defer mockReadPassword(t, "tok", nil)()

	kr := &mockKeyring{store: map[string]string{}}
	creds, err := promptAndStore(kr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.URL != "https://example.atlassian.net" {
		t.Errorf("URL = %q", creds.URL)
	}
}

func TestPromptAndStore_StoreError(t *testing.T) {
	defer pipeStdin(t, "https://example.atlassian.net", "user@example.com")()
	defer mockReadPassword(t, "tok", nil)()

	kr := &setErrKeyring{store: map[string]string{}, setErr: errors.New("store failed")}
	_, err := promptAndStore(kr)
	if err == nil {
		t.Fatal("expected error from storeCredentials failure")
	}
}
