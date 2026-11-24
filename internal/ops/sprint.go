package ops

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/termsafe"
)

// Sprint selectors accepted wherever a sprint is named, besides a numeric ID.
const (
	SprintCurrent = "current"
	SprintPrev    = "prev"
	SprintNext    = "next"
)

// StatusAdded reports issues moved into a sprint.
const StatusAdded = "added"

// boardTypeKanban is the board type without sprints. Company-managed scrum
// boards report "scrum" and team-managed boards "simple"; both can have them.
const boardTypeKanban = "kanban"

var (
	// ErrNoBoard means no board was given and the project has no sprint board.
	ErrNoBoard = errors.New("no sprint board found; pass a board ID")
	// ErrNoSprint means the selector matched no sprint on the board.
	ErrNoSprint = errors.New("no matching sprint")
	// ErrBadSprint means a sprint selector was neither a keyword nor an ID.
	ErrBadSprint = errors.New(`sprint must be "current", "prev", "next" or a sprint ID`)
	// ErrClosedSprintTarget means issues were sent to a closed sprint.
	ErrClosedSprintTarget = errors.New("can't add issues to a closed sprint")
	// ErrNoIssueKeys means an add-to-sprint request named no issues.
	ErrNoIssueKeys = errors.New("no ticket keys provided")
)

// SprintScope says which board to look at: BoardID when set, otherwise the
// only sprint board (any non-kanban board) in Project.
type SprintScope struct {
	BoardID int
	Project string
}

// ScopeFromConfig fills a scope from explicit values, falling back to the
// configured board and project.
func ScopeFromConfig(cfg config.Config, boardID int, project string) SprintScope {
	if boardID == 0 && project == "" {
		boardID = cfg.Board
	}
	if project == "" {
		project = cfg.Project
	}
	return SprintScope{BoardID: boardID, Project: project}
}

// CreateSprintScope picks the board a new ticket's sprint is looked up on:
// board when given, the configured board when the ticket is in the
// configured project, else the ticket project's only sprint board.
func CreateSprintScope(cfg config.Config, board int, project string) SprintScope {
	if board == 0 && project == cfg.Project {
		board = cfg.Board
	}
	return SprintScope{BoardID: board, Project: project}
}

// BoardChoiceError means the project has several sprint boards; Boards lists
// them so the caller can name one.
type BoardChoiceError struct {
	Project string
	Boards  []api.Board
}

func (e *BoardChoiceError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "project %s has %d sprint boards; pass a board ID:\n", e.Project, len(e.Boards))
	for _, b := range e.Boards {
		fmt.Fprintf(&sb, "- [%d] %s\n", b.ID, termsafe.Line(b.Name))
	}
	return sb.String()
}

// ResolveBoard returns the scope's board: the given ID as-is, or the
// project's only non-kanban board.
func ResolveBoard(conn api.JiraConnection, scope SprintScope) (api.Board, error) {
	if scope.BoardID != 0 {
		return api.Board{ID: scope.BoardID}, nil
	}
	boards, err := api.FetchBoards(conn, scope.Project)
	if err != nil {
		return api.Board{}, fmt.Errorf("fetching boards: %w", err)
	}
	sprintBoards := []api.Board{}
	for _, b := range boards {
		if b.Type != boardTypeKanban {
			sprintBoards = append(sprintBoards, b)
		}
	}
	switch len(sprintBoards) {
	case 0:
		return api.Board{}, ErrNoBoard
	case 1:
		return sprintBoards[0], nil
	}
	return api.Board{}, &BoardChoiceError{Project: scope.Project, Boards: sprintBoards}
}

// ListSprints returns the scope's board and its sprints, optionally filtered
// by state ("active", "future", "closed", comma-separated).
func ListSprints(conn api.JiraConnection, scope SprintScope, state string) (api.Board, []api.Sprint, error) {
	board, err := ResolveBoard(conn, scope)
	if err != nil {
		return api.Board{}, nil, err
	}
	sprints, err := api.FetchSprints(conn, board.ID, state)
	if err != nil {
		return api.Board{}, nil, fmt.Errorf("fetching sprints for board %d: %w", board.ID, err)
	}
	return board, sprints, nil
}

// FindSprint resolves a selector: a numeric sprint ID is used as-is, and
// "current", "prev" and "next" are looked up on the scope's board.
func FindSprint(conn api.JiraConnection, scope SprintScope, which string) (api.Sprint, error) {
	if id, err := strconv.Atoi(which); err == nil && id > 0 {
		return api.Sprint{ID: id}, nil
	}
	state, ok := sprintStates[which]
	if !ok {
		return api.Sprint{}, ErrBadSprint
	}
	_, sprints, err := ListSprints(conn, scope, state)
	if err != nil {
		return api.Sprint{}, err
	}
	sprint, ok := pickSprint(which, sprints)
	if !ok {
		return api.Sprint{}, fmt.Errorf("%w: no %s sprint", ErrNoSprint, which)
	}
	return sprint, nil
}

// sprintStates maps each selector keyword to the sprint state it searches.
var sprintStates = map[string]string{
	SprintCurrent: "active",
	SprintPrev:    "closed",
	SprintNext:    "future",
}

// pickSprint chooses from sprints already filtered by state: the latest
// started active sprint, the latest completed closed one, or the first
// future one (Jira lists future sprints in board order).
func pickSprint(which string, sprints []api.Sprint) (api.Sprint, bool) {
	if len(sprints) == 0 {
		return api.Sprint{}, false
	}
	best := sprints[0]
	for _, sp := range sprints[1:] {
		switch which {
		case SprintCurrent:
			if sp.StartDate > best.StartDate {
				best = sp
			}
		case SprintPrev:
			if sp.CompleteDate > best.CompleteDate {
				best = sp
			}
		}
	}
	return best, true
}
