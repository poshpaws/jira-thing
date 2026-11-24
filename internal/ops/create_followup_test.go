package ops

import (
	"errors"
	"strings"
	"testing"
)

func TestCreateTicket_SprintAndPointsAppliedAfterCreate(t *testing.T) {
	f := &fakeJira{editFields: []string{"customfield_10308"}}
	conn := startFake(t, f)
	req := CreateRequest{Project: "PROJ", Summary: "S", Sprint: "42", StoryPoints: points(3)}
	res, err := CreateTicket(conn, req, CreateOptions{})
	if err != nil || res.Key != "NEW-1" || res.Sprint.ID != 42 || res.PointsField != "customfield_10308" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(f.sprintAdds) != 1 || !strings.HasSuffix(f.sprintAdds[0], "/sprint/42/issue") {
		t.Errorf("sprint posts = %v", f.sprintAdds)
	}
	if len(f.updates) != 1 || f.updates[0]["customfield_10308"] != 3.0 {
		t.Errorf("updates = %v", f.updates)
	}
	if _, inPayload := res.Payload["customfield_10308"]; inPayload {
		t.Errorf("points must not be in the POST payload: %v", res.Payload)
	}
	if err := FollowUpError(res); err != nil {
		t.Errorf("FollowUpError = %v", err)
	}
}

func TestCreateTicket_BadSprintOrPointsCreatesNothing(t *testing.T) {
	tests := []struct {
		name string
		req  CreateRequest
		want error
	}{
		{"closed sprint", CreateRequest{Sprint: SprintPrev}, ErrClosedSprintTarget},
		{"unknown selector", CreateRequest{Sprint: "someday"}, ErrBadSprint},
		{"negative points", CreateRequest{StoryPoints: points(-1)}, ErrBadStoryPoints},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeJira{}
			tt.req.Project, tt.req.Summary = "PROJ", "S"
			_, err := CreateTicket(startFake(t, f), tt.req, CreateOptions{})
			if !errors.Is(err, tt.want) || len(f.created) != 0 {
				t.Errorf("err=%v created=%d", err, len(f.created))
			}
		})
	}
}

func TestCreateTicket_DryRunReportsSprintAndPoints(t *testing.T) {
	f := &fakeJira{}
	req := CreateRequest{Project: "PROJ", Summary: "S", Sprint: "7", StoryPoints: points(2)}
	res, err := CreateTicket(startFake(t, f), req, CreateOptions{DryRun: true})
	if err != nil || res.Sprint.ID != 7 || res.StoryPoints == nil || *res.StoryPoints != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if len(f.created)+len(f.sprintAdds)+len(f.updates) != 0 {
		t.Errorf("dry run sent requests: %+v", f)
	}
}

func TestCreateTicket_PointsFieldMissingReportedNotFatal(t *testing.T) {
	f := &fakeJira{}
	res, err := CreateTicket(startFake(t, f), CreateRequest{Project: "PROJ", Summary: "S", StoryPoints: points(1)}, CreateOptions{})
	if err != nil || res.Key != "NEW-1" || !errors.Is(res.PointsErr, ErrNoPointsField) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if err := FollowUpError(res); err == nil || !strings.Contains(err.Error(), "created NEW-1 but") {
		t.Errorf("FollowUpError = %v", err)
	}
}
