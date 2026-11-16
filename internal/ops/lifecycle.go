package ops

import (
	"fmt"

	"jira-thing/internal/api"
)

// Result statuses for lifecycle operations.
const (
	StatusCloned  = "cloned"
	StatusDeleted = "deleted"
	StatusLogged  = "logged"
)

// User identifies a Jira account.
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

// WhoAmI returns the authenticated user.
func WhoAmI(conn api.JiraConnection) (User, error) {
	self, err := api.FetchMyself(conn)
	if err != nil {
		return User{}, fmt.Errorf("fetching current user: %w", err)
	}
	return User{AccountID: str(self, "accountId"), DisplayName: str(self, "displayName"), Email: str(self, "emailAddress")}, nil
}

// fetchFields returns the "fields" object of a ticket.
func fetchFields(conn api.JiraConnection, key string) (map[string]any, error) {
	issue, err := api.FetchIssue(conn, key)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", key, err)
	}
	fields, ok := issue["fields"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("ticket %s has no fields", key)
	}
	return fields, nil
}
