package ops

import (
	"fmt"

	"jira-thing/internal/api"
)

// Watch statuses.
const (
	StatusWatching  = "watching"
	StatusUnwatched = "unwatched"
)

// WatchRequest starts or stops Who watching Key. Who is "me" (the default
// when empty), an account ID, or a name or email to look up.
type WatchRequest struct {
	Key   string
	Who   string
	Watch bool // true to watch, false to unwatch
	// Picked, when set, is used as-is instead of resolving Who.
	Picked *User
}

// WatchResult reports the change, or the request a dry run would send.
type WatchResult struct {
	Key    string
	URL    string
	User   User
	Status string
}

// WatchTicket adds or removes a watcher. With dryRun it resolves the user
// and reports the change without making it.
func WatchTicket(conn api.JiraConnection, req WatchRequest, dryRun bool) (WatchResult, error) {
	user, err := watchTarget(conn, req)
	if err != nil {
		return WatchResult{}, err
	}
	res := WatchResult{Key: req.Key, URL: IssueURL(conn, req.Key), User: user, Status: StatusUnwatched}
	if req.Watch {
		res.Status = StatusWatching
	}
	if dryRun {
		res.Status = StatusDryRun
		return res, nil
	}
	if err := sendWatch(conn, req, user.AccountID); err != nil {
		return WatchResult{}, err
	}
	return res, nil
}

// watchTarget resolves who should (un)watch, defaulting to the current user.
func watchTarget(conn api.JiraConnection, req WatchRequest) (User, error) {
	switch {
	case req.Picked != nil:
		return *req.Picked, nil
	case req.Who == "":
		return ResolveUser(conn, AssigneeMe)
	}
	return ResolveUser(conn, req.Who)
}

// sendWatch makes the add or remove call.
func sendWatch(conn api.JiraConnection, req WatchRequest, accountID string) error {
	if req.Watch {
		if err := api.AddWatcher(conn, req.Key, accountID); err != nil {
			return fmt.Errorf("watching %s: %w", req.Key, err)
		}
		return nil
	}
	if err := api.RemoveWatcher(conn, req.Key, accountID); err != nil {
		return fmt.Errorf("unwatching %s: %w", req.Key, err)
	}
	return nil
}

// WatchVerb describes a watch change for both front ends: "now watching",
// "no longer watching", or "would watch" / "would stop watching" in a dry run.
func WatchVerb(res WatchResult, watch bool) string {
	dry := res.Status == StatusDryRun
	switch {
	case watch && dry:
		return "would watch"
	case watch:
		return "now watching"
	case dry:
		return "would stop watching"
	}
	return "no longer watching"
}

// ListWatchers returns who is watching a ticket.
func ListWatchers(conn api.JiraConnection, key string) ([]User, error) {
	w, err := api.FetchWatchers(conn, key)
	if err != nil {
		return nil, fmt.Errorf("fetching watchers of %s: %w", key, err)
	}
	users := make([]User, 0, len(w.Watchers))
	for _, u := range w.Watchers {
		users = append(users, User{AccountID: u.AccountID, DisplayName: u.DisplayName, Email: u.EmailAddress})
	}
	return users, nil
}

// IsWatching reports whether the current user watches a ticket.
func IsWatching(conn api.JiraConnection, key string) (bool, error) {
	w, err := api.FetchWatchers(conn, key)
	if err != nil {
		return false, fmt.Errorf("fetching watchers of %s: %w", key, err)
	}
	return w.IsWatching, nil
}
