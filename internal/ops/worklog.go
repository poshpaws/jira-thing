package ops

import (
	"errors"
	"fmt"
	"regexp"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
)

// ErrTimeSpentRequired means a worklog had no duration.
var ErrTimeSpentRequired = errors.New("time spent is required")

// durationPattern matches Jira's duration format: one or more amounts with a
// w, d, h or m unit, e.g. "2d 3h 30m" or "1.5h".
var durationPattern = regexp.MustCompile(`^\s*([0-9]+(\.[0-9]+)?\s*[wdhm]\s*)+$`)

// WorklogRequest logs TimeSpent (Jira duration format, e.g. "2d 3h 30m")
// against Key, with an optional markdown Comment.
type WorklogRequest struct {
	Key       string
	TimeSpent string
	Comment   string
}

// WorklogResult reports the time logged, or the body a dry run would send.
type WorklogResult struct {
	Key       string
	URL       string
	TimeSpent string
	Status    string
	Payload   map[string]any
}

// LogWork records time spent against a ticket. With dryRun it checks the
// duration and returns the body it would send, without sending it.
func LogWork(conn api.JiraConnection, req WorklogRequest, dryRun bool) (WorklogResult, error) {
	if req.TimeSpent == "" {
		return WorklogResult{}, ErrTimeSpentRequired
	}
	if !durationPattern.MatchString(req.TimeSpent) {
		return WorklogResult{}, fmt.Errorf("time spent %q: use Jira's format, e.g. \"2h 30m\" or \"1d\"", req.TimeSpent)
	}
	var comment map[string]any
	if req.Comment != "" {
		comment = adf.FromMarkdown(req.Comment)
	}
	res := WorklogResult{Key: req.Key, URL: IssueURL(conn, req.Key), TimeSpent: req.TimeSpent, Status: StatusLogged,
		Payload: worklogPayload(req.TimeSpent, comment)}
	if dryRun {
		res.Status = StatusDryRun
		return res, nil
	}
	if err := api.AddWorklog(conn, req.Key, req.TimeSpent, comment); err != nil {
		return WorklogResult{}, fmt.Errorf("logging work on %s: %w", req.Key, err)
	}
	return res, nil
}

// worklogPayload mirrors the body api.AddWorklog sends.
func worklogPayload(timeSpent string, comment map[string]any) map[string]any {
	payload := map[string]any{"timeSpent": timeSpent}
	if comment != nil {
		payload["comment"] = comment
	}
	return payload
}
