package ops

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/termsafe"
)

// ErrNoPointsField means none of the story point fields is on the ticket's edit screen.
var ErrNoPointsField = errors.New(`no story point field on this ticket's edit screen; set "story_points_field" in config (find it with: diagnose -find-field "story point")`)

// ErrBadStoryPoints means a story point value was negative or not a number.
var ErrBadStoryPoints = errors.New("story points must be a number, zero or more")

// UnknownIssueTypeError means the type name isn't one the ticket can switch
// to with a plain edit. Allowed lists the ones it can.
type UnknownIssueTypeError struct {
	Key     string
	Name    string
	Allowed []string
}

func (e *UnknownIssueTypeError) Error() string {
	return fmt.Sprintf("%s can't be changed to type %q with an edit. Types it can switch to: %s.\n"+
		"Other types use a different workflow or screen; change those with Move in Jira.",
		e.Key, e.Name, strings.Join(e.Allowed, ", "))
}

// applyMetaFields resolves the issue type and story point field against the
// ticket's edit screen and adds them to res.Fields. It reads the edit
// screen only when one of them is being changed.
func applyMetaFields(conn api.JiraConnection, req UpdateRequest, res *UpdateResult) error {
	if req.IssueType == "" && req.StoryPoints == nil {
		return nil
	}
	if err := checkPoints(req.StoryPoints); err != nil {
		return err
	}
	meta, err := api.FetchEditMeta(conn, req.Key)
	if err != nil {
		return fmt.Errorf("reading %s's edit screen: %w", req.Key, err)
	}
	if req.IssueType != "" {
		if err := setIssueType(meta, req, res); err != nil {
			return err
		}
	}
	if req.StoryPoints != nil {
		return setStoryPoints(meta, req, res)
	}
	return nil
}

// checkPoints rejects a negative or non-finite story point value; nil is fine.
func checkPoints(p *float64) error {
	if p != nil && (*p < 0 || math.IsNaN(*p) || math.IsInf(*p, 0)) {
		return ErrBadStoryPoints
	}
	return nil
}

// setIssueType matches req.IssueType against the types the edit screen allows.
func setIssueType(meta map[string]api.EditField, req UpdateRequest, res *UpdateResult) error {
	allowed := meta["issuetype"].AllowedValues
	names := make([]string, 0, len(allowed))
	for _, t := range allowed {
		name := str(t, "name")
		if strings.EqualFold(name, req.IssueType) {
			res.Fields["issuetype"] = map[string]any{"id": str(t, "id")}
			res.IssueType = name
			return nil
		}
		names = append(names, name)
	}
	return &UnknownIssueTypeError{Key: req.Key, Name: termsafe.Line(req.IssueType), Allowed: termsafe.Lines(names)}
}

// setStoryPoints writes to the first candidate field on the edit screen.
func setStoryPoints(meta map[string]api.EditField, req UpdateRequest, res *UpdateResult) error {
	candidates := req.PointFields
	if candidates == nil {
		candidates = DefaultStoryPointFields
	}
	for _, field := range candidates {
		if _, ok := meta[field]; ok {
			res.Fields[field] = *req.StoryPoints
			res.PointsField = field
			return nil
		}
	}
	return fmt.Errorf("%w (tried %s)", ErrNoPointsField, strings.Join(candidates, ", "))
}
