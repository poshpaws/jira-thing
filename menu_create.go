package main

import (
	"strconv"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
)

// menuCreateTicket is the menu's "Create Ticket": it asks for an optional
// sprint and story points, then runs create's interactive flow with them.
func menuCreateTicket(api.JiraConnection) {
	sprint := promptLine(`Sprint ("current", "next" or an ID; blank for none): `)
	points := promptLine("Story points (blank for none): ")
	args, err := menuCreateArgs(sprint, points)
	if err != nil {
		printMenuErr("%v", err)
		return
	}
	runCreate(args)
}

// menuCreateArgs turns the menu's sprint and points answers into create
// flags, checking them first so a bad answer is reported in the menu rather
// than ending it.
func menuCreateArgs(sprint, points string) ([]string, error) {
	var args []string
	if sprint != "" {
		if err := checkSprintSelector(sprint); err != nil {
			return nil, err
		}
		args = append(args, "-sprint", sprint)
	}
	if points != "" {
		v, err := strconv.ParseFloat(points, 64)
		if err != nil || v < 0 {
			return nil, ops.ErrBadStoryPoints
		}
		args = append(args, "-points", points)
	}
	return args, nil
}

// checkSprintSelector accepts what a new ticket can join: "current",
// "next" or a positive sprint ID.
func checkSprintSelector(sprint string) error {
	switch sprint {
	case ops.SprintCurrent, ops.SprintNext:
		return nil
	case ops.SprintPrev:
		return ops.ErrClosedSprintTarget
	}
	if id, err := strconv.Atoi(sprint); err == nil && id > 0 {
		return nil
	}
	return ops.ErrBadSprint
}
