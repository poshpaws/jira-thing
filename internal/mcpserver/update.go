package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/termsafe"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// --- update_ticket ---

func updateTicketTool() mcp.Tool {
	return mcp.NewTool("update_ticket",
		mcp.WithDescription("Edit fields on an existing Jira ticket (summary, description, priority, labels, assignee, "+
			"parent, issue type, story points). Only the fields you provide are changed; omitted fields are left "+
			"untouched. To change workflow state, use transition_ticket instead."),
		mcp.WithString("ticket_key", mcp.Required(), mcp.Description("Jira ticket key (e.g. PROJ-42)")),
		mcp.WithString("summary", mcp.Description("New ticket summary/title")),
		mcp.WithString("description", mcp.Description("New ticket description in markdown format (replaces the existing description)")),
		mcp.WithString("priority", mcp.Description("New priority name (e.g. High, Medium, Low)")),
		mcp.WithString("labels", mcp.Description("Comma-separated list of labels (replaces all existing labels; use add_labels/remove_labels to keep the others)")),
		mcp.WithString("assignee", mcp.Description("Who to assign the ticket to: "+assigneeHelp)),
		mcp.WithString("parent", mcp.Description("Epic/parent ticket key to link this ticket to (e.g. PROJ-10)")),
		mcp.WithString("issue_type", mcp.Description("New issue type name, e.g. Task. Only types the ticket's edit screen allows; an unknown name lists them")),
		mcp.WithNumber("story_points", mcp.Description(`Story points, e.g. 3 or 0.5. Written to "story_points_field" from config, else the usual story point fields`)),
		mcp.WithString("add_labels", mcp.Description("Comma-separated labels to add, keeping the others")),
		mcp.WithString("remove_labels", mcp.Description("Comma-separated labels to remove, keeping the others")),
		mcp.WithString("add_components", mcp.Description("Comma-separated components to add")),
		mcp.WithString("remove_components", mcp.Description("Comma-separated components to remove")),
		mcp.WithString("add_fix_versions", mcp.Description("Comma-separated fix versions to add")),
		mcp.WithString("remove_fix_versions", mcp.Description("Comma-separated fix versions to remove")),
		customFieldsOption(),
		dryRunOption("resolve the fields and return the request body"),
	)
}

func handleUpdateTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		upd, err := updateRequest(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		res, err := ops.UpdateTicket(conn, upd, req.GetBool("dry_run", false))
		if err != nil {
			return mcp.NewToolResultError(termsafe.Clean(err.Error())), nil
		}
		return mcp.NewToolResultText(formatUpdate(res)), nil
	}
}

// updateRequest reads update_ticket's arguments.
func updateRequest(req mcp.CallToolRequest) (ops.UpdateRequest, error) {
	key, err := req.RequireString("ticket_key")
	if err != nil {
		return ops.UpdateRequest{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return ops.UpdateRequest{}, fmt.Errorf("loading config: %w", err)
	}
	custom, err := customFields(req)
	if err != nil {
		return ops.UpdateRequest{}, err
	}
	upd := ops.UpdateRequest{
		Key:         key,
		Summary:     req.GetString("summary", ""),
		Description: req.GetString("description", ""),
		Priority:    req.GetString("priority", ""),
		Labels:      template.SplitLabels(req.GetString("labels", "")),
		Assignee:    req.GetString("assignee", ""),
		Parent:      req.GetString("parent", ""),
		IssueType:   req.GetString("issue_type", ""),
		PointFields: ops.StoryPointFields(cfg),
		Custom:      custom,
		ListEdits: ops.ListEdits{
			Labels:      listEditArgs(req, "labels"),
			Components:  listEditArgs(req, "components"),
			FixVersions: listEditArgs(req, "fix_versions"),
		},
	}
	if _, ok := req.GetArguments()["story_points"]; ok {
		points := req.GetFloat("story_points", 0)
		upd.StoryPoints = &points
	}
	return upd, nil
}

// listEditArgs reads add_<name> and remove_<name> into list edits.
func listEditArgs(req mcp.CallToolRequest, name string) []ops.ListEdit {
	var edits []ops.ListEdit
	for _, v := range template.SplitLabels(req.GetString("add_"+name, "")) {
		edits = append(edits, ops.ListEdit{Value: v})
	}
	for _, v := range template.SplitLabels(req.GetString("remove_"+name, "")) {
		edits = append(edits, ops.ListEdit{Remove: true, Value: v})
	}
	return edits
}

// formatUpdate reports the edit, naming a new type or story points.
func formatUpdate(res ops.UpdateResult) string {
	var sb strings.Builder
	if res.Status == ops.StatusDryRun {
		fmt.Fprintf(&sb, "Dry run: would update %s with\n%s", res.Key, prettyJSON(api.EditBody(res.Fields, res.Update)))
		return sb.String()
	}
	fmt.Fprintf(&sb, "%s updated", res.Key)
	if res.IssueType != "" {
		fmt.Fprintf(&sb, "\nType: %s", termsafe.Line(res.IssueType))
	}
	if res.PointsField != "" {
		fmt.Fprintf(&sb, "\nStory points: %v (%s)", res.Fields[res.PointsField], res.PointsField)
	}
	return sb.String()
}
