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

// Credentials holds the three Jira connection values.
type Credentials struct {
	URL   string
	Email string
	Token string
}

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
func GetCredentials() (Credentials, error) {
	return getCredentials(backend)
}

// getCredentials loads credentials from kr, falling back to interactive prompt.
func getCredentials(kr Keyring) (Credentials, error) {
	url, _ := kr.Get(keyringService, keyURL)
	email, _ := kr.Get(keyringService, keyEmail)
	token, _ := kr.Get(keyringService, keyToken)
	if url == "" || email == "" || token == "" {
		return promptAndStore(kr)
	}
	return Credentials{URL: url, Email: email, Token: token}, nil
}

// promptAndStore interactively collects credentials and persists them in kr.
func promptAndStore(kr Keyring) (Credentials, error) {
	fmt.Println("Jira credentials not found. Please enter them now.")
	creds, err := readCredentials()
	if err != nil {
		return Credentials{}, err
	}
	if err := storeCredentials(kr, creds); err != nil {
		return Credentials{}, err
	}
	fmt.Println("Credentials stored securely in keyring.")
	return creds, nil
}

// readCredentials prompts stdin for URL, email, and API token.
func readCredentials() (Credentials, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Jira base URL (e.g. https://yourorg.atlassian.net): ")
	urlLine, err := reader.ReadString('\n')
	if err != nil {
		return Credentials{}, fmt.Errorf("reading URL: %w", err)
	}
	url := strings.TrimRight(strings.TrimSpace(urlLine), "/")

	fmt.Print("Jira email: ")
	emailLine, err := reader.ReadString('\n')
	if err != nil {
		return Credentials{}, fmt.Errorf("reading email: %w", err)
	}
	email := strings.TrimSpace(emailLine)

	fmt.Print("Jira API token: ")
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return Credentials{}, fmt.Errorf("reading token: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	if url == "" || email == "" || token == "" {
		return Credentials{}, fmt.Errorf("all credential fields are required")
	}
	return Credentials{URL: url, Email: email, Token: token}, nil
}

// storeCredentials writes credential values into kr.
func storeCredentials(kr Keyring, creds Credentials) error {
	if err := kr.Set(keyringService, keyURL, creds.URL); err != nil {
		return err
	}
	if err := kr.Set(keyringService, keyEmail, creds.Email); err != nil {
		return err
	}
	return kr.Set(keyringService, keyToken, creds.Token)
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
