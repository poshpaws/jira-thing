package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/tui"
)

// menuAction is a single entry in the jira-thing home menu.
type menuAction struct {
	label string
	desc  string
	run   func(conn api.JiraConnection)
}

// runMenu launches the persistent interactive menu that houses jira-cli-parity
// features. Unlike the one-shot CLI commands (which call fatal() and exit on
// error), every menu action reports errors with printMenuErr and returns
// control to the menu instead of terminating the process.
func runMenu() {
	conn := mustConnect()
	actions := menuActions()

	options := make([]tui.MenuOption, len(actions))
	for i, a := range actions {
		options[i] = tui.MenuOption{Label: a.label, Desc: a.desc}
	}

	for {
		idx, cancelled, err := selectMenuOptionFn("jira-thing menu", options)
		if err != nil {
			fatal("menu TUI: %v", err)
		}
		if cancelled {
			return
		}
		fmt.Println()
		actions[idx].run(conn)
		fmt.Println("\nPress enter to return to the menu...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n') // #nosec G104 -- pause-for-enter, error (e.g. EOF) is harmless
	}
}

// selectMenuOptionFn launches the menu-picker TUI; replaced in tests.
var selectMenuOptionFn = tui.SelectMenuOption

// showTableQuickActionsFn launches the ticket table with view/open/transition
// quick actions; replaced in tests.
var showTableQuickActionsFn = tui.ShowTableWithQuickActions

// pickTicketFn launches the ticket-picker TUI; replaced in tests.
var pickTicketFn = tui.PickTicket

// pickTicketKey shows the user's open tasks in a picker TUI, with an option to
// type a key manually for tickets not in the list. Returns "" if the user
// cancelled. Every menu action that needs a ticket key uses this instead of a
// bare text prompt, so the common case (one of your own tasks) never requires
// typing a key by hand.
func pickTicketKey(conn api.JiraConnection, label string) string {
	tickets, err := fetchTicketsByJQL(conn, buildMyTasksJQL(false))
	if err != nil {
		printMenuErr("fetching your tasks: %v", err)
		return promptLine(label)
	}
	res, err := pickTicketFn(tickets)
	if err != nil {
		printMenuErr("TUI: %v", err)
		return ""
	}
	if res.Cancelled {
		return ""
	}
	if res.Manual {
		return promptLine(label)
	}
	return res.Key
}

func menuActions() []menuAction {
	return []menuAction{
		{"My Tasks", "List open tasks assigned to you", menuMyTasks},
		{"Search", "Search tickets by JQL", menuSearch},
		{"Describe Ticket", "View full ticket details", menuDescribe},
		{"Create Ticket", "Create a ticket from a template", func(api.JiraConnection) { runCreate(nil) }},
		{"Change State", "Move a ticket to a new workflow state", func(api.JiraConnection) { runState(nil) }},
		{"Update Fields", "Edit summary, priority, labels, or assignee", menuUpdateFields},
		{"Add Comment", "Add a comment to a ticket", menuAddComment},
		{"Add Worklog", "Log time spent against a ticket", menuAddWorklog},
		{"Attach File", "Attach a local file to a ticket", menuAttach},
		{"Create Subtask", "Create a subtask under a ticket", menuCreateSubtask},
		{"Link Tickets", "Link two tickets together", menuLinkTickets},
		{"Unlink Tickets", "Remove a link between two tickets", menuUnlinkTickets},
		{"Clone Ticket", "Clone an existing ticket", menuCloneTicket},
		{"Delete Ticket", "Permanently delete a ticket", menuDeleteTicket},
		{"Boards", "List Agile boards", menuListBoards},
		{"Sprints", "List sprints on a board and their issues", menuListSprints},
		{"Projects", "List accessible projects", menuListProjects},
		{"Releases", "List a project's releases/versions", menuListVersions},
		{"List Epics", "List every epic in a project", menuListEpics},
		{"Describe Epic", "List the issues under an epic", menuDescribeEpic},
		{"Create Epic", "Create a new epic", menuCreateEpic},
		{"Add to Epic", "Add issues to an epic", menuAddToEpic},
		{"Remove from Epic", "Remove issues from their epic", menuRemoveFromEpic},
		{"Open in Browser", "Open a ticket (or Jira) in the browser", menuOpen},
		{"Who Am I", "Show the current authenticated user", menuWhoami},
		{"Confluence Browse", "Browse the Confluence space tree", func(api.JiraConnection) { runConfBrowse() }},
	}
}

// printMenuErr reports a menu action failure without exiting the process.
func printMenuErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", tui.ErrorStyle.Render("Error:"), fmt.Sprintf(format, args...))
}

// promptLine reads a single trimmed line from stdin with a label prompt.
func promptLine(label string) string {
	fmt.Print(label)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line)
}

// promptConfirm asks a yes/no question, defaulting to no on empty input.
func promptConfirm(label string) bool {
	answer := strings.ToLower(promptLine(label + " [y/N]: "))
	return answer == "y" || answer == "yes"
}

func menuMyTasks(conn api.JiraConnection) {
	fetch := func() ([]tui.Ticket, error) {
		return fetchTicketsByJQL(conn, buildMyTasksJQL(false))
	}
	tickets, err := fetch()
	if err != nil {
		printMenuErr("fetching tasks: %v", err)
		return
	}
	if len(tickets) == 0 {
		fmt.Println("No tasks found.")
		return
	}
	browseWithQuickActions(conn, tickets, fetch)
}

func menuSearch(conn api.JiraConnection) {
	jql := promptLine("JQL: ")
	if jql == "" {
		printMenuErr("JQL is required")
		return
	}
	fetch := func() ([]tui.Ticket, error) { return fetchTicketsByJQL(conn, jql) }
	tickets, err := fetch()
	if err != nil {
		printMenuErr("searching: %v", err)
		return
	}
	if len(tickets) == 0 {
		fmt.Println("No tickets found.")
		return
	}
	browseWithQuickActions(conn, tickets, fetch)
}

// fetchTicketsByJQL runs a JQL search and converts the results to tui.Ticket rows.
func fetchTicketsByJQL(conn api.JiraConnection, jql string) ([]tui.Ticket, error) {
	result, err := api.SearchIssues(conn, api.SearchQuery{
		JQL:        jql,
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	})
	if err != nil {
		return nil, err
	}
	return issuesToTickets(result.Issues), nil
}

// browseWithQuickActions shows tickets in the interactive table and performs
// whichever quick action (view/open/transition/copy) the user triggered on exit.
// fetch, if non-nil, backs the table's ctrl+r refresh.
func browseWithQuickActions(conn api.JiraConnection, tickets []tui.Ticket, fetch tui.TicketFetcher) {
	res, err := showTableQuickActionsFn(tickets, fetch)
	if err != nil {
		printMenuErr("TUI: %v", err)
		return
	}
	switch res.Action {
	case tui.ActionView:
		issue, err := api.FetchIssue(conn, res.Key)
		if err != nil {
			printMenuErr("fetching %s: %v", res.Key, err)
			return
		}
		renderDescribe(issue)
	case tui.ActionOpen:
		target := conn.BaseURL + "/browse/" + res.Key
		if err := openBrowser(target); err != nil {
			printMenuErr("opening browser: %v", err)
			return
		}
		fmt.Printf("Opened %s\n", target)
	case tui.ActionTransition:
		menuTransitionTicket(conn, res.Key)
	case tui.ActionCopyKey:
		if err := copyToClipboard(res.Key); err != nil {
			printMenuErr("copying key: %v", err)
			return
		}
		fmt.Printf("Copied %s to clipboard\n", res.Key)
	case tui.ActionCopyURL:
		target := conn.BaseURL + "/browse/" + res.Key
		if err := copyToClipboard(target); err != nil {
			printMenuErr("copying url: %v", err)
			return
		}
		fmt.Printf("Copied %s to clipboard\n", target)
	}
}

// menuTransitionTicket runs the same transition flow as the `state` command,
// inline for a ticket key already known (e.g. from a quick action).
func menuTransitionTicket(conn api.JiraConnection, key string) {
	transitions, err := api.FetchTransitions(conn, key)
	if err != nil {
		printMenuErr("fetching transitions for %s: %v", key, err)
		return
	}
	if len(transitions) == 0 {
		fmt.Printf("No transitions available for %s.\n", key)
		return
	}
	picked, cancelled, err := selectTransitionFn(key, transitionsToOptions(transitions))
	if err != nil {
		printMenuErr("TUI: %v", err)
		return
	}
	if cancelled {
		fmt.Println("Cancelled.")
		return
	}
	if err := api.TransitionIssue(conn, key, picked.ID); err != nil {
		printMenuErr("transitioning %s: %v", key, err)
		return
	}
	fmt.Printf("%s moved to %s\n", key, picked.Name)
}

func menuDescribe(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	issue, err := api.FetchIssue(conn, key)
	if err != nil {
		printMenuErr("fetching %s: %v", key, err)
		return
	}
	renderDescribe(issue)
}

func menuUpdateFields(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	fields := map[string]any{}
	if v := promptLine("New summary (blank to skip): "); v != "" {
		fields["summary"] = v
	}
	if v := promptLine("New priority (blank to skip): "); v != "" {
		fields["priority"] = map[string]any{"name": v}
	}
	if v := promptLine("New labels, comma-separated (blank to skip): "); v != "" {
		fields["labels"] = splitCommaList(v)
	}
	if len(fields) == 0 {
		fmt.Println("Nothing to update.")
		return
	}
	if err := api.UpdateIssue(conn, key, fields); err != nil {
		printMenuErr("updating %s: %v", key, err)
		return
	}
	fmt.Printf("%s updated\n", key)
}

func menuAddComment(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	comment := promptLine("Comment: ")
	if comment == "" {
		printMenuErr("comment is empty")
		return
	}
	if err := api.AddComment(conn, key, buildDescription(comment)); err != nil {
		printMenuErr("adding comment to %s: %v", key, err)
		return
	}
	fmt.Printf("Comment added to %s\n", key)
}

func menuAddWorklog(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	timeSpent := promptLine("Time spent (e.g. 2h, 1d 3h): ")
	if timeSpent == "" {
		printMenuErr("time spent is required")
		return
	}
	var comment map[string]any
	if c := promptLine("Comment (blank to skip): "); c != "" {
		comment = buildDescription(c)
	}
	if err := api.AddWorklog(conn, key, timeSpent, comment); err != nil {
		printMenuErr("logging work on %s: %v", key, err)
		return
	}
	fmt.Printf("Logged %s on %s\n", timeSpent, key)
}

func menuAttach(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	path := promptLine("File path: ")
	if _, err := api.AddAttachment(conn, key, path); err != nil {
		printMenuErr("attaching %s to %s: %v", path, key, err)
		return
	}
	fmt.Printf("Attached %s to %s\n", path, key)
}

func menuCreateSubtask(conn api.JiraConnection) {
	parentKey := pickTicketKey(conn, "Parent ticket key: ")
	if parentKey == "" {
		fmt.Println("Cancelled.")
		return
	}
	summary := promptLine("Subtask summary: ")
	if summary == "" {
		printMenuErr("summary is required")
		return
	}
	description := promptLine("Description (blank to skip): ")

	issue, err := api.FetchIssue(conn, parentKey)
	if err != nil {
		printMenuErr("fetching %s: %v", parentKey, err)
		return
	}
	parentFields, _ := issue["fields"].(map[string]any)
	fields := buildSubtaskFields(parentKey, inheritedFields(parentFields), parsedTask{Summary: summary, Description: description})
	result, err := api.CreateIssue(conn, fields)
	if err != nil {
		printMenuErr("creating subtask: %v", err)
		return
	}
	fmt.Printf("Created subtask %s under %s\n", getString(result, "key"), parentKey)
}

func menuLinkTickets(conn api.JiraConnection) {
	types, err := api.FetchIssueLinkTypes(conn)
	if err != nil {
		printMenuErr("fetching link types: %v", err)
		return
	}
	fmt.Println("Available link types:")
	for _, t := range types {
		fmt.Printf("  - %s\n", t.Name)
	}
	outward := pickTicketKey(conn, "Outward ticket key: ")
	if outward == "" {
		fmt.Println("Cancelled.")
		return
	}
	inward := pickTicketKey(conn, "Inward ticket key: ")
	if inward == "" {
		fmt.Println("Cancelled.")
		return
	}
	linkType := promptLine("Link type: ")
	if err := api.LinkIssues(conn, outward, inward, linkType); err != nil {
		printMenuErr("linking: %v", err)
		return
	}
	fmt.Printf("Linked %s -> %s (%s)\n", outward, inward, linkType)
}

func menuUnlinkTickets(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	other := pickTicketKey(conn, "Other ticket key: ")
	if other == "" {
		fmt.Println("Cancelled.")
		return
	}
	if err := api.UnlinkIssues(conn, key, other); err != nil {
		printMenuErr("unlinking: %v", err)
		return
	}
	fmt.Printf("Unlinked %s from %s\n", key, other)
}

func menuCloneTicket(conn api.JiraConnection) {
	sourceKey := pickTicketKey(conn, "Source ticket key: ")
	if sourceKey == "" {
		fmt.Println("Cancelled.")
		return
	}
	issue, err := api.FetchIssue(conn, sourceKey)
	if err != nil {
		printMenuErr("fetching %s: %v", sourceKey, err)
		return
	}
	source, _ := issue["fields"].(map[string]any)
	fields := inheritedCloneFields(source)
	summary := promptLine("Summary (blank for default): ")
	if summary == "" {
		summary = "CLONE - " + getString(source, "summary")
	}
	fields["summary"] = summary
	result, err := api.CreateIssue(conn, fields)
	if err != nil {
		printMenuErr("cloning: %v", err)
		return
	}
	fmt.Printf("Cloned %s -> %s\n", sourceKey, getString(result, "key"))
}

func menuDeleteTicket(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key to delete: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	if !promptConfirm(fmt.Sprintf("Permanently delete %s?", key)) {
		fmt.Println("Cancelled.")
		return
	}
	cascade := promptConfirm("Also delete its subtasks?")
	if err := api.DeleteIssue(conn, key, cascade); err != nil {
		printMenuErr("deleting %s: %v", key, err)
		return
	}
	fmt.Printf("Deleted %s\n", key)
}

func menuListBoards(conn api.JiraConnection) {
	projectKey := promptLine("Project key (blank for all): ")
	boards, err := api.FetchBoards(conn, projectKey)
	if err != nil {
		printMenuErr("fetching boards: %v", err)
		return
	}
	if len(boards) == 0 {
		fmt.Println("No boards found.")
		return
	}
	for _, b := range boards {
		fmt.Printf("  [%d] %s (%s)\n", b.ID, b.Name, b.Type)
	}
}

func menuListSprints(conn api.JiraConnection) {
	boardIDStr := promptLine("Board ID: ")
	boardID, err := strconv.Atoi(boardIDStr)
	if err != nil {
		printMenuErr("invalid board ID: %v", err)
		return
	}
	sprints, err := api.FetchSprints(conn, boardID, "")
	if err != nil {
		printMenuErr("fetching sprints: %v", err)
		return
	}
	if len(sprints) == 0 {
		fmt.Println("No sprints found.")
		return
	}
	for _, sp := range sprints {
		fmt.Printf("  [%d] %s (%s)\n", sp.ID, sp.Name, sp.State)
	}
	sprintIDStr := promptLine("View issues in sprint ID (blank to skip): ")
	if sprintIDStr == "" {
		return
	}
	sprintID, err := strconv.Atoi(sprintIDStr)
	if err != nil {
		printMenuErr("invalid sprint ID: %v", err)
		return
	}
	result, err := api.FetchSprintIssues(conn, sprintID, "")
	if err != nil {
		printMenuErr("fetching sprint issues: %v", err)
		return
	}
	printTasks(result.Issues)
}

func menuListProjects(conn api.JiraConnection) {
	projects, err := api.FetchProjects(conn)
	if err != nil {
		printMenuErr("fetching projects: %v", err)
		return
	}
	for _, p := range projects {
		fmt.Printf("  %s — %s\n", p.Key, p.Name)
	}
}

func menuListVersions(conn api.JiraConnection) {
	projectKey := promptLine("Project key: ")
	versions, err := api.FetchProjectVersions(conn, projectKey)
	if err != nil {
		printMenuErr("fetching versions: %v", err)
		return
	}
	if len(versions) == 0 {
		fmt.Println("No versions found.")
		return
	}
	for _, v := range versions {
		fmt.Printf("  %s (released: %v)\n", v.Name, v.Released)
	}
}

func menuListEpics(conn api.JiraConnection) {
	projectKey := promptLine("Project key (blank for config default): ")
	if projectKey == "" {
		cfg, err := config.Load()
		if err != nil || cfg.Project == "" {
			printMenuErr("no project specified and no default project in config")
			return
		}
		projectKey = cfg.Project
	}
	result, err := api.FetchEpics(conn, projectKey)
	if err != nil {
		printMenuErr("listing epics: %v", err)
		return
	}
	if len(result.Issues) == 0 {
		fmt.Printf("No epics found in %s.\n", projectKey)
		return
	}
	printTasks(result.Issues)
}

func menuDescribeEpic(conn api.JiraConnection) {
	epicKey := pickTicketKey(conn, "Epic key: ")
	if epicKey == "" {
		fmt.Println("Cancelled.")
		return
	}
	result, err := api.ListEpicIssues(conn, epicKey)
	if err != nil {
		printMenuErr("listing epic issues: %v", err)
		return
	}
	if len(result.Issues) == 0 {
		fmt.Println("No issues found under this epic.")
		return
	}
	printTasks(result.Issues)
}

func menuCreateEpic(conn api.JiraConnection) {
	project := promptLine("Project key: ")
	name := promptLine("Epic name: ")
	summary := promptLine("Epic summary: ")
	if project == "" || name == "" || summary == "" {
		printMenuErr("project, name, and summary are all required")
		return
	}
	fields := api.BuildEpicFields(conn, project, name, summary)
	result, err := api.CreateIssue(conn, fields)
	if err != nil {
		printMenuErr("creating epic: %v", err)
		return
	}
	fmt.Printf("Created epic %s\n", getString(result, "key"))
}

func menuAddToEpic(conn api.JiraConnection) {
	epicKey := pickTicketKey(conn, "Epic key: ")
	if epicKey == "" {
		fmt.Println("Cancelled.")
		return
	}
	keys := splitCommaList(promptLine("Ticket keys, comma-separated: "))
	if len(keys) == 0 {
		printMenuErr("at least one ticket key is required")
		return
	}
	if err := api.AddIssuesToEpic(conn, epicKey, keys); err != nil {
		printMenuErr("%v", err)
		return
	}
	fmt.Printf("Added %d ticket(s) to %s\n", len(keys), epicKey)
}

func menuRemoveFromEpic(conn api.JiraConnection) {
	keys := splitCommaList(promptLine("Ticket keys to remove from their epic, comma-separated: "))
	if len(keys) == 0 {
		printMenuErr("at least one ticket key is required")
		return
	}
	if err := api.RemoveIssuesFromEpic(conn, keys); err != nil {
		printMenuErr("%v", err)
		return
	}
	fmt.Printf("Removed %d ticket(s) from their epic\n", len(keys))
}

func menuOpen(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key (blank to open Jira home): ")
	target := conn.BaseURL
	if key != "" {
		target = conn.BaseURL + "/browse/" + key
	}
	if err := openBrowser(target); err != nil {
		printMenuErr("opening browser: %v", err)
		return
	}
	fmt.Printf("Opened %s\n", target)
}

func menuWhoami(conn api.JiraConnection) {
	self, err := api.FetchMyself(conn)
	if err != nil {
		printMenuErr("fetching current user: %v", err)
		return
	}
	fmt.Printf("Account ID: %v\nDisplay name: %v\nEmail: %v\n",
		self["accountId"], self["displayName"], self["emailAddress"])
}

// splitCommaList splits a comma-separated string into trimmed, non-empty parts.
func splitCommaList(s string) []string {
	parts := strings.Split(s, ",")
	trimmed := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			trimmed = append(trimmed, t)
		}
	}
	return trimmed
}

// inheritedFields extracts the subtask-inheritable fields from a parent issue's fields map.
func inheritedFields(fields map[string]any) map[string]any {
	inherited := make(map[string]any)
	for _, key := range subtaskInheritFields {
		if v, ok := fields[key]; ok && v != nil {
			inherited[key] = v
		}
	}
	return inherited
}

// inheritedCloneFields extracts the clone-inheritable fields from a source issue's fields map.
func inheritedCloneFields(fields map[string]any) map[string]any {
	keys := []string{"project", "issuetype", "priority", "labels", "components", "description"}
	inherited := make(map[string]any, len(keys))
	for _, key := range keys {
		if v, ok := fields[key]; ok && v != nil {
			inherited[key] = v
		}
	}
	return inherited
}
