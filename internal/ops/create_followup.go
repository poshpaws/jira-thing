package ops

import (
	"errors"
	"fmt"
	"strconv"

	"jira-thing/internal/api"
	"jira-thing/internal/termsafe"
)

// followUps are the changes a create makes after the POST, because Jira's
// create screen often lacks the fields: joining a sprint and setting story
// points through the new ticket's edit screen.
type followUps struct {
	sprint      api.Sprint // zero ID: no sprint
	points      *float64
	pointFields []string
}

// planFollowUps validates the story points and resolves the sprint before
// anything is created, so a bad value fails without leaving a ticket behind.
func planFollowUps(conn api.JiraConnection, req CreateRequest, fields map[string]any) (followUps, error) {
	if err := checkPoints(req.StoryPoints); err != nil {
		return followUps{}, err
	}
	plan := followUps{points: req.StoryPoints, pointFields: req.PointFields}
	if req.Sprint == "" {
		return plan, nil
	}
	if req.Sprint == SprintPrev {
		return followUps{}, ErrClosedSprintTarget
	}
	scope := req.SprintScope
	if scope == (SprintScope{}) {
		scope.Project = FieldsProject(fields)
	}
	sprint, err := FindSprint(conn, scope, req.Sprint)
	if err != nil {
		return followUps{}, fmt.Errorf("finding sprint %q: %w", req.Sprint, err)
	}
	plan.sprint = sprint
	return plan, nil
}

// report records the planned follow-ups on a create result.
func (p followUps) report(res *CreateResult) {
	res.Sprint = p.sprint
	res.StoryPoints = p.points
}

// apply runs the follow-ups against the newly created res.Key, recording
// failures on res rather than returning them: the ticket already exists.
func (p followUps) apply(conn api.JiraConnection, res *CreateResult) {
	p.report(res)
	if p.sprint.ID != 0 {
		res.SprintErr = sendToSprint(conn, p.sprint.ID, []string{res.Key})
	}
	if p.points == nil {
		return
	}
	upd, err := UpdateTicket(conn, UpdateRequest{Key: res.Key, StoryPoints: p.points, PointFields: p.pointFields}, false)
	res.PointsField, res.PointsErr = upd.PointsField, err
}

// SprintLabel names a sprint by name and ID, or by ID alone when it was
// given as a number and never looked up.
func SprintLabel(sp api.Sprint) string {
	if sp.Name == "" {
		return strconv.Itoa(sp.ID)
	}
	return fmt.Sprintf("%s (%d)", termsafe.Line(sp.Name), sp.ID)
}

// FollowUpError reports a ticket that was created but whose parent link,
// sprint or story points could not be set; nil when all of them succeeded.
func FollowUpError(res CreateResult) error {
	var errs []error
	if res.ParentErr != nil {
		errs = append(errs, fmt.Errorf("failed to link to parent %s: %w", res.Parent, res.ParentErr))
	}
	if res.SprintErr != nil {
		errs = append(errs, fmt.Errorf("failed to add to sprint %d: %w", res.Sprint.ID, res.SprintErr))
	}
	if res.PointsErr != nil {
		errs = append(errs, fmt.Errorf("failed to set story points: %w", res.PointsErr))
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("created %s but %w", res.Key, errors.Join(errs...))
}
