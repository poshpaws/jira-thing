package ops

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/termsafe"
)

// userSearchLimit caps how many candidates a name lookup considers.
const userSearchLimit = 10

// accountIDPattern matches Jira Cloud account IDs: the legacy 24-hex form and
// the "prefix:id" forms (e.g. "557058:f58131cb-...", "qm:..."). Anything else
// is treated as a name or email to look up.
var accountIDPattern = regexp.MustCompile(`^([0-9a-f]{24}|[0-9A-Za-z]+:[0-9A-Za-z:-]+)$`)

// ErrUserNotFound means a name or email matched no active Jira user.
var ErrUserNotFound = errors.New("no active Jira user matches")

// AmbiguousUserError means a name matched several users; Candidates lists them
// so the caller can pick one, or retry with an email or account ID.
type AmbiguousUserError struct {
	Query      string
	Candidates []User
}

func (e *AmbiguousUserError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%q matches %d users; use an email or account ID:\n", e.Query, len(e.Candidates))
	for _, u := range e.Candidates {
		fmt.Fprintf(&sb, "- %s\n", UserLabel(u))
	}
	return sb.String()
}

// UserLabel renders a user on one line as "Name <email> (accountId)", leaving out the
// email when Jira's privacy settings hide it.
func UserLabel(u User) string {
	name, id := termsafe.Line(u.DisplayName), termsafe.Line(u.AccountID)
	if u.Email == "" {
		return fmt.Sprintf("%s (%s)", name, id)
	}
	return fmt.Sprintf("%s <%s> (%s)", name, termsafe.Line(u.Email), id)
}

// SearchUsers returns the active human users whose name or email matches query.
func SearchUsers(conn api.JiraConnection, query string) ([]User, error) {
	found, err := api.SearchUsers(conn, query, userSearchLimit)
	if err != nil {
		return nil, fmt.Errorf("searching users for %q: %w", query, err)
	}
	users := []User{}
	for _, u := range found {
		if u.Active && u.AccountType == "atlassian" {
			users = append(users, User{AccountID: u.AccountID, DisplayName: u.DisplayName, Email: u.EmailAddress})
		}
	}
	return users, nil
}

// ResolveUser turns "me", an account ID, a name or an email into one user.
// A name matching several users returns an *AmbiguousUserError.
func ResolveUser(conn api.JiraConnection, who string) (User, error) {
	switch {
	case who == AssigneeMe:
		return WhoAmI(conn)
	case accountIDPattern.MatchString(who):
		return User{AccountID: who}, nil
	}
	users, err := SearchUsers(conn, who)
	if err != nil {
		return User{}, err
	}
	return pickUser(who, users)
}

// pickUser returns the only candidate, or the only one whose name or email
// matches query exactly, ignoring case.
func pickUser(query string, users []User) (User, error) {
	switch len(users) {
	case 0:
		return User{}, fmt.Errorf("%w %q", ErrUserNotFound, query)
	case 1:
		return users[0], nil
	}
	var exact []User
	for _, u := range users {
		if strings.EqualFold(u.DisplayName, query) || strings.EqualFold(u.Email, query) {
			exact = append(exact, u)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	return User{}, &AmbiguousUserError{Query: query, Candidates: users}
}
