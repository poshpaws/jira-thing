package mcpserver

import (
	"fmt"

	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"

	"github.com/mark3labs/mcp-go/mcp"
)

// createFollowUpOptions are create_ticket's arguments applied once the
// ticket exists: its sprint and story points.
func createFollowUpOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("sprint",
			mcp.Description(`Sprint to add the new ticket to: "current" (active), "next" (first future) or a sprint ID`),
		),
		mcp.WithNumber("board_id",
			mcp.Description("Board the sprint is looked up on (from list_boards). Default: the configured board for its project, else the project's only sprint board"),
		),
		mcp.WithNumber("story_points",
			mcp.Description(`Story points, e.g. 3 or 0.5, set after the create. Written to "story_points_field" from config, else the usual story point fields`),
		),
	}
}

// addCreateFollowUps reads sprint, board_id and story_points into createReq.
func addCreateFollowUps(req mcp.CallToolRequest, createReq *ops.CreateRequest, cfg config.Config) {
	createReq.Sprint = req.GetString("sprint", "")
	if _, hasPoints := req.GetArguments()["story_points"]; hasPoints {
		points := req.GetFloat("story_points", 0)
		createReq.StoryPoints = &points
		createReq.PointFields = ops.StoryPointFields(cfg)
	}
	if createReq.Sprint != "" {
		createReq.SprintScope = ops.CreateSprintScope(cfg, int(req.GetFloat("board_id", 0)), createReq.Project)
	}
}

// formatCreateFollowUps words the parent link, sprint and story point
// outcomes of a create, one line each.
func formatCreateFollowUps(res ops.CreateResult) string {
	return formatParentLink(res) + formatSprintAdd(res) + formatPointsSet(res)
}

// formatParentLink words the parent link outcome, empty when none was asked for.
func formatParentLink(res ops.CreateResult) string {
	switch {
	case res.Parent == "":
		return ""
	case res.ParentErr != nil:
		return fmt.Sprintf("\nWarning: created but failed to link to parent %s: %s", termsafe.Line(res.Parent), termsafe.Clean(res.ParentErr.Error()))
	}
	return "\nLinked to parent: " + termsafe.Line(res.Parent)
}

// formatSprintAdd words the sprint outcome, empty when none was asked for.
func formatSprintAdd(res ops.CreateResult) string {
	switch {
	case res.Sprint.ID == 0:
		return ""
	case res.SprintErr != nil:
		return fmt.Sprintf("\nWarning: created but failed to add to sprint %d: %s", res.Sprint.ID, termsafe.Clean(res.SprintErr.Error()))
	}
	return "\nAdded to sprint: " + ops.SprintLabel(res.Sprint)
}

// formatPointsSet words the story point outcome, empty when none was asked for.
func formatPointsSet(res ops.CreateResult) string {
	switch {
	case res.StoryPoints == nil:
		return ""
	case res.PointsErr != nil:
		return "\nWarning: created but failed to set story points: " + termsafe.Clean(res.PointsErr.Error())
	}
	return fmt.Sprintf("\nStory points: %v (%s)", *res.StoryPoints, res.PointsField)
}
