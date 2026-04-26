package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	gokeyring "github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	keyringService = "jira-thing-poc"
	keyURL         = "jira_url"
	keyEmail       = "jira_email"
	keyToken       = "jira_api_token" // #nosec G101 -- keyring lookup key, not a credential value
)

// Credentials holds the three Jira connection values.
type Credentials struct {
	URL   string
	Email string
	Token string
}

// readPassword reads a password from the terminal. Overridden in tests.
var readPassword = func() ([]byte, error) {
	return term.ReadPassword(int(os.Stdin.Fd()))
}

// Keyring abstracts credential storage to allow test injection.
type Keyring interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

// systemKeyring delegates to the OS keyring via go-keyring.
type systemKeyring struct{}

func (systemKeyring) Get(key string) (string, error)        { return gokeyring.Get(keyringService, key) }
func (systemKeyring) Set(key, value string) error           { return gokeyring.Set(keyringService, key, value) }
func (systemKeyring) Delete(key string) error               { return gokeyring.Delete(keyringService, key) }

// backend is the active keyring implementation; replaced in tests.
var backend Keyring = systemKeyring{}

// GetCredentials returns stored Jira credentials, prompting the user if any are missing.
func GetCredentials() (Credentials, error) {
	return getCredentials(backend)
}

// getCredentials loads credentials from kr, falling back to interactive prompt.
func getCredentials(kr Keyring) (Credentials, error) {
	url, err := kr.Get(keyURL)
	if err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
		return Credentials{}, fmt.Errorf("reading URL from keyring: %w", err)
	}
	email, err := kr.Get(keyEmail)
	if err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
		return Credentials{}, fmt.Errorf("reading email from keyring: %w", err)
	}
	token, err := kr.Get(keyToken)
	if err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
		return Credentials{}, fmt.Errorf("reading token from keyring: %w", err)
	}
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
	tokenBytes, err := readPassword()
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
	if err := kr.Set(keyURL, creds.URL); err != nil {
		return err
	}
	if err := kr.Set(keyEmail, creds.Email); err != nil {
		return err
	}
	return kr.Set(keyToken, creds.Token)
}

// ClearCredentials removes all stored Jira credentials from the keyring.
func ClearCredentials() error {
	return clearCredentials(backend)
}

// clearCredentials deletes all credential keys from kr; not-found errors are ignored.
func clearCredentials(kr Keyring) error {
	for _, key := range []string{keyURL, keyEmail, keyToken} {
		if err := kr.Delete(key); err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
			return fmt.Errorf("clearing %s: %w", key, err)
		}
	}
	fmt.Println("Credentials cleared.")
	return nil
}
