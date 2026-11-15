package api

import (
	"errors"
	"testing"
)

func TestValidateIssueKey(t *testing.T) {
	for _, ok := range []string{"PROJ-42", "ab_1-7", "10023"} {
		if err := ValidateIssueKey(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "PROJ", "../../myself", "PROJ-1/comment", "PROJ-1?fields=*all", "PROJ-1#x", "-1", "PROJ-1 "} {
		if err := ValidateIssueKey(bad); !errors.Is(err, ErrInvalidIssueKey) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestFetchIssue_RejectsCraftedKeyBeforeAnyRequest(t *testing.T) {
	_, err := FetchIssue(JiraConnection{BaseURL: "https://unused.invalid"}, "../../myself")
	if !errors.Is(err, ErrInvalidIssueKey) {
		t.Errorf("got %v", err)
	}
}
