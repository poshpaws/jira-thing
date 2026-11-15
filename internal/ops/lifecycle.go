package ops

import (
	"errors"
	"fmt"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
)

// Result statuses for lifecycle operations.
const (
	StatusCloned  = "cloned"
	StatusDeleted = "deleted"
	StatusLogged  = "logged"
)

// ClonePrefix is prepended to the source summary when a clone gets no summary of its own.
const ClonePrefix = "CLONE - "

// cloneInheritFields are copied from the source ticket before any overrides.
var cloneInheritFields = []string{"project", "issuetype", "priority", "labels", "components", "description"}

// ErrTimeSpentRequired means a worklog had no duration.
var ErrTimeSpentRequired = errors.New("time spent is required")

// CloneRequest copies SourceKey into a new ticket. Empty overrides keep the
// source's values; an empty Summary becomes ClonePrefix + the source summary.
type CloneRequest struct {
	SourceKey string
	Summary   string
	Priority  string
	Assignee  string // "me", "unassign" or an account ID
}

// CloneResult reports the new ticket.
type CloneResult struct {
	SourceKey string
	Key       string
	URL       string
	Status    string
}

// CloneTicket creates a copy of a ticket, inheriting its project, type,
// priority, labels, components and description.
func CloneTicket(conn api.JiraConnection, req CloneRequest) (CloneResult, error) {
	source, err := fetchFields(conn, req.SourceKey)
	if err != nil {
		return CloneResult{}, err
	}
	inherited := map[string]any{}
	for _, k := range cloneInheritFields {
		if v, ok := source[k]; ok && v != nil {
			inherited[k] = v
		}
	}
	summary := req.Summary
	if summary == "" {
		summary = ClonePrefix + str(source, "summary")
	}
	created, err := CreateTicket(conn, CreateRequest{
		Fields: inherited, Summary: summary, Priority: req.Priority, Assignee: req.Assignee,
	}, CreateOptions{})
	if err != nil {
		return CloneResult{}, fmt.Errorf("cloning %s: %w", req.SourceKey, err)
	}
	return CloneResult{SourceKey: req.SourceKey, Key: created.Key, URL: created.URL, Status: StatusCloned}, nil
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

// DeleteTicket permanently deletes a ticket; cascade also deletes its subtasks.
func DeleteTicket(conn api.JiraConnection, key string, cascade bool) error {
	if err := api.DeleteIssue(conn, key, cascade); err != nil {
		return fmt.Errorf("deleting %s: %w", key, err)
	}
	return nil
}

// WorklogRequest logs TimeSpent (Jira duration format, e.g. "2d 3h 30m")
// against Key, with an optional markdown Comment.
type WorklogRequest struct {
	Key       string
	TimeSpent string
	Comment   string
}

// LogWork records time spent against a ticket.
func LogWork(conn api.JiraConnection, req WorklogRequest) error {
	if req.TimeSpent == "" {
		return ErrTimeSpentRequired
	}
	var comment map[string]any
	if req.Comment != "" {
		comment = adf.FromMarkdown(req.Comment)
	}
	if err := api.AddWorklog(conn, req.Key, req.TimeSpent, comment); err != nil {
		return fmt.Errorf("logging work on %s: %w", req.Key, err)
	}
	return nil
}

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
