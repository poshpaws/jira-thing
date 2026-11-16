package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/profile"

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

func (systemKeyring) Get(key string) (string, error) { return gokeyring.Get(keyringService, key) }
func (systemKeyring) Set(key, value string) error    { return gokeyring.Set(keyringService, key, value) }
func (systemKeyring) Delete(key string) error        { return gokeyring.Delete(keyringService, key) }

// backend is the active keyring implementation; replaced in tests.
var backend Keyring = systemKeyring{}

// GetCredentials returns stored Jira credentials, prompting the user if any are missing.
func GetCredentials() (Credentials, error) {
	return getCredentials(backend)
}

// credentialKeys are the keyring keys for one profile's credentials.
type credentialKeys struct{ url, email, token string }

// activeKeys returns the active profile's keyring keys: the original ones
// for the default profile, "key@name" for a named one.
func activeKeys() (credentialKeys, error) {
	name, err := profile.Active()
	if err != nil {
		return credentialKeys{}, err
	}
	return credentialKeys{profile.Key(keyURL, name), profile.Key(keyEmail, name), profile.Key(keyToken, name)}, nil
}

// getCredentials uses the environment override when set, else loads the
// active profile's credentials from kr, prompting for any that are missing.
func getCredentials(kr Keyring) (Credentials, error) {
	if creds, ok, err := envCredentials(); ok || err != nil {
		return creds, err
	}
	keys, err := activeKeys()
	if err != nil {
		return Credentials{}, err
	}
	creds, err := readStored(kr, keys)
	if err != nil {
		return Credentials{}, err
	}
	if creds.URL == "" || creds.Email == "" || creds.Token == "" {
		return promptAndStore(kr)
	}
	return creds, nil
}

// readStored reads one profile's credentials; missing keys come back empty.
func readStored(kr Keyring, keys credentialKeys) (Credentials, error) {
	var creds Credentials
	for _, item := range []struct {
		key, what string
		dest      *string
	}{{keys.url, "URL", &creds.URL}, {keys.email, "email", &creds.Email}, {keys.token, "token", &creds.Token}} {
		v, err := kr.Get(item.key)
		if err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
			return Credentials{}, fmt.Errorf("reading %s from keyring: %w", item.what, err)
		}
		*item.dest = v
	}
	return creds, nil
}

// promptAndStore interactively collects credentials and persists them in kr.
func promptAndStore(kr Keyring) (Credentials, error) {
	name, err := profile.Active()
	if err != nil {
		return Credentials{}, err
	}
	fmt.Printf("Jira credentials for profile %q not found. Please enter them now.\n", profile.Label(name))
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

	fmt.Print("Jira API token (input not echoed): ")
	tokenBytes, err := readPassword()
	fmt.Println()
	if err != nil {
		return Credentials{}, fmt.Errorf("reading token: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	if url == "" || email == "" || token == "" {
		return Credentials{}, fmt.Errorf("all credential fields are required")
	}
	if err := ValidateToken(token); err != nil {
		return Credentials{}, err
	}
	return Credentials{URL: url, Email: email, Token: token}, nil
}

// ValidateToken checks for whitespace and double-paste by length.
func ValidateToken(t string) error {
	if strings.ContainsAny(t, " \t\n\r") {
		return fmt.Errorf("token contains whitespace — paste carefully")
	}
	if len(t) > 250 {
		return fmt.Errorf("token too long — may have been pasted twice")
	}
	return nil
}

// storeCredentials writes credential values into kr under the active profile's keys.
func storeCredentials(kr Keyring, creds Credentials) error {
	keys, err := activeKeys()
	if err != nil {
		return err
	}
	if err := kr.Set(keys.url, creds.URL); err != nil {
		return err
	}
	if err := kr.Set(keys.email, creds.Email); err != nil {
		return err
	}
	return kr.Set(keys.token, creds.Token)
}

// ClearCredentials removes all stored Jira credentials from the keyring.
func ClearCredentials() error {
	return clearCredentials(backend)
}

// clearCredentials deletes the active profile's credential keys from kr;
// not-found errors are ignored. Other profiles are left alone.
func clearCredentials(kr Keyring) error {
	keys, err := activeKeys()
	if err != nil {
		return err
	}
	for _, key := range []string{keys.url, keys.email, keys.token} {
		if err := kr.Delete(key); err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
			return fmt.Errorf("clearing %s: %w", key, err)
		}
	}
	name, _ := profile.Active()
	fmt.Printf("Credentials cleared for profile %q.\n", profile.Label(name))
	return nil
}
