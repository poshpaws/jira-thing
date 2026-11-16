package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// sprintHelp describes the sprint selector argument.
const sprintHelp = `Sprint: "current" (active), "prev" (last closed), "next" (first future) or a sprint ID`

func registerAgileTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(listBoardsTool(), handleListBoards(conn))
	s.AddTool(listSprintsTool(), handleListSprints(conn))
	s.AddTool(listSprintIssuesTool(), handleListSprintIssues(conn))
	s.AddTool(addToSprintTool(), handleAddToSprint(conn))
}

// boardOptions are the arguments every sprint tool takes to pick a board.
func boardOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithNumber("board_id",
			mcp.Description("Board ID (from list_boards). Default: the configured board, else the project's only sprint board"),
		),
		mcp.WithString("project_key",
			mcp.Description("Project whose sprint board to use when board_id is not given. Default: the configured project"),
		),
	}
}

// sprintScope reads board_id and project_key, falling back to the config file.
func sprintScope(req mcp.CallToolRequest) (ops.SprintScope, error) {
	cfg, err := config.Load()
	if err != nil {
		return ops.SprintScope{}, fmt.Errorf("loading config: %w", err)
	}
	return ops.ScopeFromConfig(cfg, int(req.GetFloat("board_id", 0)), req.GetString("project_key", "")), nil
}

// sprintErrorText points the agent at board_id when the project has several
// sprint boards.
func sprintErrorText(err error) string {
	var choice *ops.BoardChoiceError
	if errors.As(err, &choice) {
		return err.Error() + "\nRetry with board_id set to one of the boards above."
	}
	return err.Error()
}

// sprintLabel renders a sprint as "[id] name (state, start → end)".
func sprintLabel(sp api.Sprint) string {
	label := fmt.Sprintf("[%d] %s (%s", sp.ID, termsafe.Line(sp.Name), termsafe.Line(sp.State))
	if sp.StartDate != "" {
		label += fmt.Sprintf(", %s → %s", termsafe.Line(dateOnly(sp.StartDate)), termsafe.Line(dateOnly(sp.EndDate)))
	}
	return label + ")"
}

// dateOnly trims an ISO 8601 timestamp to its date.
func dateOnly(ts string) string {
	if len(ts) > len("2006-01-02") {
		return ts[:len("2006-01-02")]
	}
	return ts
}

// --- list_boards ---

func listBoardsTool() mcp.Tool {
	return mcp.NewTool("list_boards",
		mcp.WithDescription("List Agile (Scrum/Kanban) boards visible to the current user, optionally filtered to a project."),
		mcp.WithString("project_key",
			mcp.Description("Optional Jira project key to filter boards to (e.g. PROJ)"),
		),
	)
}

func handleListBoards(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boards, err := ops.ListBoards(conn, req.GetString("project_key", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(boards) == 0 {
			return mcp.NewToolResultText("No boards found."), nil
		}
		var sb strings.Builder
		for _, b := range boards {
			fmt.Fprintf(&sb, "- [%d] %s (%s)\n", b.ID, termsafe.Line(b.Name), termsafe.Line(b.Type))
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- list_sprints ---

func listSprintsTool() mcp.Tool {
	opts := append([]mcp.ToolOption{
		mcp.WithDescription("List the sprints on a board, oldest first."),
		mcp.WithString("state",
			mcp.Description("Optional comma-separated filter: any of \"active\", \"future\", \"closed\""),
		),
	}, boardOptions()...)
	return mcp.NewTool("list_sprints", opts...)
}

func handleListSprints(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		scope, err := sprintScope(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		board, sprints, err := ops.ListSprints(conn, scope, req.GetString("state", ""))
		if err != nil {
			return mcp.NewToolResultError(sprintErrorText(err)), nil
		}
		return mcp.NewToolResultText(formatSprints(board, sprints)), nil
	}
}

// formatSprints lists a board's sprints, one per line.
func formatSprints(board api.Board, sprints []api.Sprint) string {
	if len(sprints) == 0 {
		return fmt.Sprintf("No sprints found on board %d.", board.ID)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Sprints on board %s:\n", boardName(board))
	for _, sp := range sprints {
		fmt.Fprintf(&sb, "- %s\n", sprintLabel(sp))
	}
	return sb.String()
}

// boardName is "[id] name", or just the ID when the board was given by ID.
func boardName(b api.Board) string {
	if b.Name == "" {
		return strconv.Itoa(b.ID)
	}
	return fmt.Sprintf("[%d] %s", b.ID, termsafe.Line(b.Name))
}
