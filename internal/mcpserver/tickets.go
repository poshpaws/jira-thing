package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"jira-thing/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTicketTools(s *server.MCPServer, conn api.JiraConnection) {
	s.AddTool(describeTicketTool(), handleDescribeTicket(conn))
	s.AddTool(searchTicketsTool(), handleSearchTickets(conn))
	s.AddTool(myTasksTool(), handleMyTasks(conn))
	s.AddTool(lastCommentTool(), handleLastComment(conn))
	s.AddTool(addCommentTool(), handleAddComment(conn))
	s.AddTool(createTicketTool(), handleCreateTicket(conn))
	s.AddTool(updateTicketTool(), handleUpdateTicket(conn))
	s.AddTool(listTransitionsTool(), handleListTransitions(conn))
	s.AddTool(transitionTicketTool(), handleTransitionTicket(conn))
}

// --- describe_ticket ---

func describeTicketTool() mcp.Tool {
	return mcp.NewTool("describe_ticket",
		mcp.WithDescription("Fetch a Jira ticket's full details including summary, status, priority, assignee, and description"),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
	)
}

func handleDescribeTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		issue, err := api.FetchIssue(conn, key)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching %s: %v", key, err)), nil
		}
		return mcp.NewToolResultText(formatIssue(issue)), nil
	}
}

// --- search_tickets ---

func searchTicketsTool() mcp.Tool {
	return mcp.NewTool("search_tickets",
		mcp.WithDescription("Search Jira tickets using JQL. Returns key, summary, status, priority, and updated date for each match."),
		mcp.WithString("jql",
			mcp.Required(),
			mcp.Description("JQL query string (e.g. 'project = PROJ AND status = \"In Progress\"')"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of results to return (default 20, max 100)"),
		),
	)
}

func handleSearchTickets(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jql, err := req.RequireString("jql")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		maxResults := int(req.GetFloat("max_results", 20))
		if maxResults < 1 || maxResults > 100 {
			maxResults = 20
		}
		result, err := api.SearchIssues(conn, api.SearchQuery{
			JQL:        jql,
			Fields:     []string{"summary", "status", "priority", "updated", "assignee"},
			MaxResults: maxResults,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}
		return mcp.NewToolResultText(formatSearchResults(result)), nil
	}
}

// --- my_tasks ---

func myTasksTool() mcp.Tool {
	return mcp.NewTool("my_tasks",
		mcp.WithDescription("List open Jira tasks assigned to the current user"),
		mcp.WithBoolean("stale_only",
			mcp.Description("If true, only show tasks with no updates in the last 3 business days"),
		),
	)
}

func handleMyTasks(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		staleOnly := req.GetBool("stale_only", false)
		jql := `assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC`
		if staleOnly {
			jql = `assignee = currentUser() AND resolution = Unresolved AND updated <= -3d ORDER BY updated ASC`
		}
		result, err := api.SearchIssues(conn, api.SearchQuery{
			JQL:        jql,
			Fields:     []string{"summary", "status", "priority", "updated"},
			MaxResults: 50,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching tasks: %v", err)), nil
		}
		if len(result.Issues) == 0 {
			return mcp.NewToolResultText("No open tasks found."), nil
		}
		return mcp.NewToolResultText(formatSearchResults(result)), nil
	}
}

// --- last_comment ---

func lastCommentTool() mcp.Tool {
	return mcp.NewTool("last_comment",
		mcp.WithDescription("Fetch the most recent comment on a Jira ticket, rendered as markdown"),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
	)
}

func handleLastComment(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		comment, err := api.FetchLastComment(conn, key)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching comment: %v", err)), nil
		}
		return mcp.NewToolResultText(formatComment(comment)), nil
	}
}

// --- add_comment ---

func addCommentTool() mcp.Tool {
	return mcp.NewTool("add_comment",
		mcp.WithDescription("Add a markdown comment to a Jira ticket. The markdown is converted to Atlassian Document Format automatically."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("comment",
			mcp.Required(),
			mcp.Description("Comment text in markdown format"),
		),
	)
}

func handleAddComment(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		comment, err := req.RequireString("comment")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if strings.TrimSpace(comment) == "" {
			return mcp.NewToolResultError("comment is empty"), nil
		}
		adf := markdownToADFBody(comment)
		if err := api.AddComment(conn, key, adf); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("adding comment: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Comment added to %s", key)), nil
	}
}

// --- create_ticket ---

func createTicketTool() mcp.Tool {
	return mcp.NewTool("create_ticket",
		mcp.WithDescription("Create a new Jira ticket with the specified fields"),
		mcp.WithString("project",
			mcp.Required(),
			mcp.Description("Project key (e.g. PROJ)"),
		),
		mcp.WithString("summary",
			mcp.Required(),
			mcp.Description("Ticket summary/title"),
		),
		mcp.WithString("issue_type",
			mcp.Description("Issue type name (default: Task)"),
		),
		mcp.WithString("priority",
			mcp.Description("Priority name (e.g. High, Medium, Low)"),
		),
		mcp.WithString("description",
			mcp.Description("Ticket description in markdown format"),
		),
		mcp.WithString("labels",
			mcp.Description("Comma-separated list of labels"),
		),
	)
}

func handleCreateTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := req.RequireString("project")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		fields := map[string]any{
			"project":   map[string]any{"key": project},
			"summary":   summary,
			"issuetype": map[string]any{"name": req.GetString("issue_type", "Task")},
		}

		if priority := req.GetString("priority", ""); priority != "" {
			fields["priority"] = map[string]any{"name": priority}
		}
		if desc := req.GetString("description", ""); desc != "" {
			fields["description"] = markdownToADFBody(desc)
		}
		if labelsStr := req.GetString("labels", ""); labelsStr != "" {
			fields["labels"] = splitLabels(labelsStr)
		}

		result, err := api.CreateIssue(conn, fields)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating ticket: %v", err)), nil
		}
		key := ""
		if k, ok := result["key"].(string); ok {
			key = k
		}
		return mcp.NewToolResultText(fmt.Sprintf("Created ticket: %s\nURL: %s/browse/%s", key, conn.BaseURL, key)), nil
	}
}

// --- update_ticket ---

func updateTicketTool() mcp.Tool {
	return mcp.NewTool("update_ticket",
		mcp.WithDescription("Edit fields on an existing Jira ticket (summary, description, priority, labels, assignee). "+
			"Only the fields you provide are changed; omitted fields are left untouched. To change workflow state, use "+
			"transition_ticket instead."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("summary",
			mcp.Description("New ticket summary/title"),
		),
		mcp.WithString("description",
			mcp.Description("New ticket description in markdown format (replaces the existing description)"),
		),
		mcp.WithString("priority",
			mcp.Description("New priority name (e.g. High, Medium, Low)"),
		),
		mcp.WithString("labels",
			mcp.Description("Comma-separated list of labels (replaces all existing labels)"),
		),
		mcp.WithString("assignee",
			mcp.Description("Who to assign the ticket to: \"me\" for the current user, \"unassign\" to clear the assignee, "+
				"or a Jira account ID"),
		),
	)
}

func handleUpdateTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		fields, err := buildUpdateFields(conn, req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(fields) == 0 {
			return mcp.NewToolResultError("no fields provided to update"), nil
		}
		if err := api.UpdateIssue(conn, key, fields); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating %s: %v", key, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s updated", key)), nil
	}
}

// buildUpdateFields translates the update_ticket request arguments into a Jira fields payload.
func buildUpdateFields(conn api.JiraConnection, req mcp.CallToolRequest) (map[string]any, error) {
	fields := map[string]any{}
	if summary := req.GetString("summary", ""); summary != "" {
		fields["summary"] = summary
	}
	if desc := req.GetString("description", ""); desc != "" {
		fields["description"] = markdownToADFBody(desc)
	}
	if priority := req.GetString("priority", ""); priority != "" {
		fields["priority"] = map[string]any{"name": priority}
	}
	if labelsStr := req.GetString("labels", ""); labelsStr != "" {
		fields["labels"] = splitLabels(labelsStr)
	}
	if assignee := req.GetString("assignee", ""); assignee != "" {
		resolved, err := resolveAssignee(conn, assignee)
		if err != nil {
			return nil, err
		}
		fields["assignee"] = resolved
	}
	return fields, nil
}

// resolveAssignee turns the assignee argument into the Jira API's expected assignee field value.
func resolveAssignee(conn api.JiraConnection, assignee string) (any, error) {
	switch assignee {
	case "unassign":
		return nil, nil
	case "me":
		self, err := api.FetchMyself(conn)
		if err != nil {
			return nil, fmt.Errorf("resolving current user: %w", err)
		}
		return map[string]any{"accountId": self["accountId"]}, nil
	default:
		return map[string]any{"accountId": assignee}, nil
	}
}

func splitLabels(labelsStr string) []string {
	labels := strings.Split(labelsStr, ",")
	trimmed := make([]string, 0, len(labels))
	for _, l := range labels {
		if t := strings.TrimSpace(l); t != "" {
			trimmed = append(trimmed, t)
		}
	}
	return trimmed
}

// --- list_transitions ---

func listTransitionsTool() mcp.Tool {
	return mcp.NewTool("list_transitions",
		mcp.WithDescription("List the workflow states a Jira ticket can currently move to. Boards customise workflows "+
			"heavily (a personal board may have 4 states, a client board 12), so always call this before transition_ticket "+
			"rather than assuming a state name is valid."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
	)
}

func handleListTransitions(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		transitions, err := api.FetchTransitions(conn, key)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching transitions for %s: %v", key, err)), nil
		}
		if len(transitions) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No transitions available for %s.", key)), nil
		}
		return mcp.NewToolResultText(formatTransitions(key, transitions)), nil
	}
}

// --- transition_ticket ---

func transitionTicketTool() mcp.Tool {
	return mcp.NewTool("transition_ticket",
		mcp.WithDescription("Move a Jira ticket to a new workflow state. The target_status must match one of the "+
			"states returned by list_transitions for this ticket — call that tool first if unsure what states are available."),
		mcp.WithString("ticket_key",
			mcp.Required(),
			mcp.Description("Jira ticket key (e.g. PROJ-42)"),
		),
		mcp.WithString("target_status",
			mcp.Required(),
			mcp.Description("Name of the state to move to (e.g. \"In Progress\"), matched case-insensitively against list_transitions output"),
		),
	)
}

func handleTransitionTicket(conn api.JiraConnection) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key, err := req.RequireString("ticket_key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		targetStatus, err := req.RequireString("target_status")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		transitions, err := api.FetchTransitions(conn, key)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching transitions for %s: %v", key, err)), nil
		}
		transition, err := findTransition(transitions, targetStatus)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("%v\n\n%s", err, formatTransitions(key, transitions))), nil
		}
		if err := api.TransitionIssue(conn, key, transition.ID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("transitioning %s: %v", key, err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s moved to %s", key, transition.Name)), nil
	}
}

// findTransition matches targetStatus against a transition's name (case-insensitive).
func findTransition(transitions []api.Transition, targetStatus string) (api.Transition, error) {
	for _, t := range transitions {
		if strings.EqualFold(t.Name, targetStatus) {
			return t, nil
		}
	}
	return api.Transition{}, fmt.Errorf("no transition to %q is available from the current state", targetStatus)
}

func formatTransitions(key string, transitions []api.Transition) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Available states for %s:\n\n", key)
	for _, t := range transitions {
		fmt.Fprintf(&sb, "- %s\n", t.Name)
	}
	return sb.String()
}

// --- formatters ---

func formatIssue(issue map[string]any) string {
	var sb strings.Builder
	key, _ := issue["key"].(string)
	fields, _ := issue["fields"].(map[string]any)
	if fields == nil {
		fields = map[string]any{}
	}

	summary, _ := fields["summary"].(string)
	fmt.Fprintf(&sb, "# %s: %s\n\n", key, summary)

	if status, ok := fields["status"].(map[string]any); ok {
		fmt.Fprintf(&sb, "**Status:** %s\n", status["name"])
	}
	if priority, ok := fields["priority"].(map[string]any); ok {
		fmt.Fprintf(&sb, "**Priority:** %s\n", priority["name"])
	}
	if assignee, ok := fields["assignee"].(map[string]any); ok {
		fmt.Fprintf(&sb, "**Assignee:** %s\n", assignee["displayName"])
	}
	if updated, ok := fields["updated"].(string); ok && len(updated) >= 10 {
		fmt.Fprintf(&sb, "**Updated:** %s\n", updated[:10])
	}
	if labels, ok := fields["labels"].([]any); ok && len(labels) > 0 {
		labelStrs := make([]string, 0, len(labels))
		for _, l := range labels {
			if s, ok := l.(string); ok {
				labelStrs = append(labelStrs, s)
			}
		}
		fmt.Fprintf(&sb, "**Labels:** %s\n", strings.Join(labelStrs, ", "))
	}

	sb.WriteString("\n")

	if desc, ok := fields["description"].(map[string]any); ok {
		sb.WriteString("## Description\n\n")
		sb.WriteString(adfToMarkdownBody(desc))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatSearchResults(result api.SearchResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d ticket(s):\n\n", result.Total)
	for _, issue := range result.Issues {
		key, _ := issue["key"].(string)
		f, _ := issue["fields"].(map[string]any)
		if f == nil {
			f = map[string]any{}
		}
		summary, _ := f["summary"].(string)
		status := nestedStr(f, "status", "name")
		priority := nestedStr(f, "priority", "name")
		updated, _ := f["updated"].(string)
		if len(updated) >= 10 {
			updated = updated[:10]
		}
		fmt.Fprintf(&sb, "- **%s** [%s] %s — %s (updated: %s)\n", key, status, priority, summary, updated)
	}
	return sb.String()
}

func formatComment(comment api.Comment) string {
	var sb strings.Builder
	author := ""
	if a, ok := comment.Author["displayName"].(string); ok {
		author = a
	}
	created := comment.Created
	if len(created) >= 10 {
		created = created[:10]
	}
	fmt.Fprintf(&sb, "**%s** on %s:\n\n", author, created)
	if comment.Body != nil {
		sb.WriteString(adfToMarkdownBody(comment.Body))
	}
	return sb.String()
}

func nestedStr(m map[string]any, key1, key2 string) string {
	if inner, ok := m[key1].(map[string]any); ok {
		if v, ok := inner[key2].(string); ok {
			return v
		}
	}
	return ""
}

// markdownToADFBody converts markdown text to ADF document format.
// This calls back into the main package's converter via a function variable
// set during server initialisation.
var markdownToADFBody func(md string) map[string]any

// adfToMarkdownBody converts an ADF document node to markdown text.
var adfToMarkdownBody func(node map[string]any) string

// SetConverters wires up the markdown/ADF conversion functions.
// Called from main before starting the server, since the converters
// live in package main and cannot be imported.
// Panics if either function is nil — callers must wire both before serving.
func SetConverters(toADF func(string) map[string]any, toMD func(map[string]any) string) {
	if toADF == nil || toMD == nil {
		panic("mcpserver: SetConverters called with nil function")
	}
	markdownToADFBody = toADF
	adfToMarkdownBody = toMD
}
