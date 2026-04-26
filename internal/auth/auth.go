package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	gokeyring "github.com/zalando/go-keyring"
)

const (
	keyringService = "jira-thing-poc"
	keyURL         = "jira_url"
	keyEmail       = "jira_email"
	keyToken       = "jira_api_token"
)

// Keyring abstracts credential storage to allow test injection.
type Keyring interface {
	Get(service, key string) (string, error)
	Set(service, key, value string) error
	Delete(service, key string) error
}

// systemKeyring delegates to the OS keyring via go-keyring.
type systemKeyring struct{}

func (systemKeyring) Get(service, key string) (string, error) { return gokeyring.Get(service, key) }
func (systemKeyring) Set(service, key, value string) error    { return gokeyring.Set(service, key, value) }
func (systemKeyring) Delete(service, key string) error        { return gokeyring.Delete(service, key) }

// backend is the active keyring implementation; replaced in tests.
var backend Keyring = systemKeyring{}

// GetCredentials returns stored Jira credentials, prompting the user if any are missing.
func GetCredentials() (url, email, token string, err error) {
	return getCredentials(backend)
}

// getCredentials loads credentials from kr, falling back to interactive prompt.
func getCredentials(kr Keyring) (url, email, token string, err error) {
	url, _ = kr.Get(keyringService, keyURL)
	email, _ = kr.Get(keyringService, keyEmail)
	token, _ = kr.Get(keyringService, keyToken)
	if url == "" || email == "" || token == "" {
		return promptAndStore(kr)
	}
	return url, email, token, nil
}

// promptAndStore interactively collects credentials and persists them in kr.
func promptAndStore(kr Keyring) (string, string, string, error) {
	fmt.Println("Jira credentials not found. Please enter them now.")
	url, email, token, err := readCredentials()
	if err != nil {
		return "", "", "", err
	}
	if err := storeCredentials(kr, url, email, token); err != nil {
		return "", "", "", err
	}
	fmt.Println("Credentials stored securely in keyring.")
	return url, email, token, nil
}

// readCredentials prompts stdin for URL, email, and API token.
func readCredentials() (url, email, token string, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Jira base URL (e.g. https://yourorg.atlassian.net): ")
	url, _ = reader.ReadString('\n')
	url = strings.TrimRight(strings.TrimSpace(url), "/")

	fmt.Print("Jira email: ")
	email, _ = reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Jira API token: ")
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", "", "", fmt.Errorf("reading token: %w", err)
	}
	token = strings.TrimSpace(string(tokenBytes))

	if url == "" || email == "" || token == "" {
		return "", "", "", fmt.Errorf("all credential fields are required")
	}
	return url, email, token, nil
}

// storeCredentials writes all three credential values into kr.
func storeCredentials(kr Keyring, url, email, token string) error {
	if err := kr.Set(keyringService, keyURL, url); err != nil {
		return err
	}
	if err := kr.Set(keyringService, keyEmail, email); err != nil {
		return err
	}
	return kr.Set(keyringService, keyToken, token)
}

// ClearCredentials removes all stored Jira credentials from the keyring.
func ClearCredentials() {
	clearCredentials(backend)
}

// clearCredentials deletes all credential keys from kr, ignoring missing-key errors.
func clearCredentials(kr Keyring) {
	for _, key := range []string{keyURL, keyEmail, keyToken} {
		_ = kr.Delete(keyringService, key)
	}
	fmt.Println("Credentials cleared.")
}
