package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"jira-thing/internal/api"
	"jira-thing/internal/auth"
	"jira-thing/internal/config"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
	vercheck "jira-thing/internal/version"
)

// version is set at build time via -ldflags.
var version = "dev"

// getCredentialsFn resolves Jira credentials; replaced in tests.
var getCredentialsFn = auth.GetCredentials

// showTableFn launches the interactive TUI; replaced in tests.
var showTableFn = tui.ShowTable

// osExit calls os.Exit; replaced in tests to prevent process termination.
var osExit = os.Exit

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println("jira-thing " + version)
	case "check-update", "cu":
		fmt.Println(vercheck.CheckMessage(version))
	case "help", "--help", "-h":
		printUsage()
	case "template", "te":
		runTemplate(os.Args[2:])
	case "create", "cr":
		runCreate(os.Args[2:])
	case "my-tasks", "mt":
		runMyTasks(os.Args[2:])
	case "update", "up":
		runUpdate(os.Args[2:])
	case "last-comment", "lc":
		runLastComment(os.Args[2:])
	case "clear-auth":
		if err := auth.ClearCredentials(); err != nil {
			fatal("clearing credentials: %v", err)
		}
	case "toil-check", "toil", "tc":
		runToilCheck()
	case "toil-sync", "ts":
		runToilSync()
	case "diagnose", "diag":
		runDiagnose(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// printUsage writes the styled command summary to stderr.
func printUsage() {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Render("jira-thing") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(" "+version)
	usage := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")).Render("Usage: ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("jira-thing") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(" <command> [options]")

	cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	commands := []struct{ cmd, desc string }{
		{"template|te <TICKET-KEY> [-o file]", "Fetch a ticket and save as template"},
		{"create|cr   [-t template.json]    ", "Create a new ticket from a template"},
		{"update|up   <TICKET-KEY> [-stdin] ", "Add a comment via $EDITOR or stdin"},
		{"my-tasks|mt [-notupdated]         ", "List open tasks assigned to you"},
		{"last-comment|lc <TICKET-KEY>      ", "Show last comment as markdown"},
		{"toil-check|tc                     ", "List toil tickets from the last week"},
		{"toil-sync|ts                      ", "Sync TOIL tickets to Confluence"},
		{"diagnose|diag                     ", "Test API connectivity and credentials"},
		{"clear-auth                        ", "Clear stored credentials"},
		{"check-update|cu                    ", "Check for newer releases on GitHub"},
		{"version|--version|-v              ", "Show version"},
	}

	fmt.Fprintf(os.Stderr, "\n  %s\n\n", title)
	fmt.Fprintf(os.Stderr, "  %s\n\n", usage)
	fmt.Fprintf(os.Stderr, "  %s\n\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("Commands:"))
	for _, c := range commands {
		fmt.Fprintf(os.Stderr, "    %s  %s\n", cmdStyle.Render(c.cmd), descStyle.Render(c.desc))
	}
	fmt.Fprintln(os.Stderr)
}

// buildConnection resolves credentials and returns a JiraConnection.
func buildConnection() (api.JiraConnection, error) {
	creds, err := getCredentialsFn()
	if err != nil {
		return api.JiraConnection{}, err
	}
	return api.JiraConnection{BaseURL: creds.URL, Email: creds.Email, APIToken: creds.Token}, nil
}

// mustConnect calls buildConnection and exits on failure.
func mustConnect() api.JiraConnection {
	conn, err := buildConnection()
	if err != nil {
		fatal("connecting: %v", err)
	}
	return conn
}

// fatal prints an error to stderr and exits with code 1.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	osExit(1)
}

// runTemplate fetches a Jira ticket and saves it as a local JSON template.
func runTemplate(args []string) {
	// Reorder args so flags precede positional arguments for flag.Parse.
	var reordered, positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			reordered = append(reordered, args[i], args[i+1])
			i++
		} else if strings.HasPrefix(args[i], "-") {
			reordered = append(reordered, args[i])
		} else {
			positional = append(positional, args[i])
		}
	}
	reordered = append(reordered, positional...)

	fs := flag.NewFlagSet("template", flag.ContinueOnError)
	output := fs.String("o", "", "Output file path")
	if err := fs.Parse(reordered); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing template <TICKET-KEY> [-o output.json]")
	}
	conn := mustConnect()
	fmt.Printf("Fetching %s...\n", fs.Arg(0))
	issue, err := api.FetchIssue(conn, fs.Arg(0))
	if err != nil {
		fatal("fetching issue: %v", err)
	}
	tmpl := template.Build(issue)
	promptAssigneeChoice(tmpl)
	saved, err := template.Save(tmpl, *output)
	if err != nil {
		fatal("saving template: %v", err)
	}
	fmt.Printf("Template saved to %s\n", saved)
	out, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		fatal("marshalling template: %v", err)
	}
	fmt.Println(string(out))
}

// runCreate loads a template, prompts for summary/description, and creates a Jira ticket.
func runCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	templatePath := fs.String("t", "", "Path to template file")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	tmpl, err := template.Load(*templatePath)
	if err != nil {
		if *templatePath == "" {
			fatal("no template found in search path.\n\nCreate one first:\n  jira-thing template <TICKET-KEY> [-o ticket_template.json]\n\nThen place ticket_template.json in one of:\n  - current directory\n  - same directory as the jira-thing binary\n  - $XDG_CONFIG_HOME/jira-thing/\n  - ~/.config/jira-thing/\n\nOr specify a path directly:\n  jira-thing create -t /path/to/template.json")
		}
		fatal("loading template: %v", err)
	}
	summary, description, err := promptTicketFields()
	if err != nil {
		fatal("%v", err)
	}
	tmpl["summary"] = summary
	tmpl["description"] = buildDescription(description)
	template.StripExcludedFields(tmpl)

	conn := mustConnect()

	if tmpl["assignee"] == template.AssigneeSelf {
		me, err := api.FetchMyself(conn)
		if err != nil {
			fatal("fetching current user: %v", err)
		}
		template.ResolveAssignee(tmpl, me["accountId"].(string))
	}

	result, err := api.CreateIssue(conn, tmpl)
	if err != nil {
		fatal("creating issue: %v", err)
	}
	key := getString(result, "key")
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Created ticket:"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

// promptTicketFields reads summary and description from stdin.
func promptTicketFields() (summary, description string, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter ticket summary: ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("reading summary: %w", err)
	}
	summary = strings.TrimSpace(line)
	if summary == "" {
		return "", "", fmt.Errorf("summary is required")
	}
	fmt.Print("Enter ticket description: ")
	line, err = reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("reading description: %w", err)
	}
	return summary, strings.TrimSpace(line), nil
}

// promptAssigneeChoice asks the user whether to use self or the original assignee in the template.
// If no assignee is present, it defaults to self without prompting.
func promptAssigneeChoice(tmpl map[string]any) {
	assignee, ok := tmpl["assignee"].(map[string]any)
	if !ok {
		tmpl["assignee"] = template.AssigneeSelf
		return
	}
	displayName, _ := assignee["displayName"].(string)
	if displayName == "" {
		displayName = "original user"
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Assignee: (1) current user (self) or (2) %s? [1/2]: ", displayName)
	line, _ := reader.ReadString('\n')
	if strings.TrimSpace(line) == "2" {
		fmt.Printf("Keeping assignee: %s\n", displayName)
		return
	}
	tmpl["assignee"] = template.AssigneeSelf
	fmt.Println("Assignee set to current user (resolved at ticket creation time)")
}

// buildDescription wraps plain text in the Jira Atlassian Document Format structure.
func buildDescription(text string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": text}},
			},
		},
	}
}

// runMyTasks lists open Jira tasks assigned to the current user.
// With -notupdated, restricts to tasks idle for 3+ business days.
func runMyTasks(args []string) {
	fs := flag.NewFlagSet("my-tasks", flag.ExitOnError)
	notUpdated := fs.Bool("notupdated", false, "Only show tasks with no updates in the last 3 business days")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	conn := mustConnect()
	q := api.SearchQuery{
		JQL:        buildMyTasksJQL(*notUpdated),
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	}
	result, err := api.SearchIssues(conn, q)
	if err != nil {
		fatal("fetching tasks: %v", err)
	}
	if len(result.Issues) == 0 {
		fmt.Println("No tasks found.")
		return
	}
	fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Found %d %s task(s):", len(result.Issues), taskLabel(*notUpdated))))
	fmt.Println()
	printTasks(result.Issues)
}

// issuesToTickets converts Jira search results to TUI tickets.
func issuesToTickets(issues []map[string]any) []tui.Ticket {
	tickets := make([]tui.Ticket, 0, len(issues))
	for _, issue := range issues {
		f, _ := issue["fields"].(map[string]any)
		if f == nil {
			f = map[string]any{}
		}
		tickets = append(tickets, tui.Ticket{
			Key:      getString(issue, "key"),
			Status:   nestedString(f, "status", "name"),
			Priority: nestedString(f, "priority", "name"),
			Updated:  getString(f, "updated"),
			Summary:  getString(f, "summary"),
		})
	}
	return tickets
}

// buildMyTasksJQL constructs the JQL for the my-tasks query.
// When notUpdated is true, it adds an upper bound on the updated date.
func buildMyTasksJQL(notUpdated bool) string {
	base := `assignee = currentUser() AND resolution = Unresolved`
	if notUpdated {
		cutoff := threeBusinessDaysAgo(time.Now())
		return fmt.Sprintf(`%s AND updated <= "%s" ORDER BY updated ASC`, base, cutoff.Format("2006/01/02"))
	}
	return base + ` ORDER BY updated DESC`
}

// taskLabel returns a human-readable description for the filter mode.
func taskLabel(notUpdated bool) string {
	if notUpdated {
		return "stale (no updates in 3+ business days)"
	}
	return "open"
}

// printTasks renders each issue as a formatted row.
func printTasks(issues []map[string]any) {
	for _, issue := range issues {
		printTaskRow(issue)
	}
}

// printTaskRow prints a single issue line: key, status, priority, last-updated date, summary.
func printTaskRow(issue map[string]any) {
	key := getString(issue, "key")
	f, ok := issue["fields"].(map[string]any)
	if !ok {
		f = map[string]any{}
	}
	summary := getString(f, "summary")
	updated := getString(f, "updated")
	if len(updated) >= 10 {
		updated = updated[:10]
	}
	fmt.Printf("  %s  %s  %s  %s  %s\n",
		tui.KeyStyle.Render(fmt.Sprintf("%-12s", key)),
		tui.StatusStyle.Render(fmt.Sprintf("%-14s", nestedString(f, "status", "name"))),
		tui.PriorityStyle.Render(fmt.Sprintf("%-8s", nestedString(f, "priority", "name"))),
		tui.DateStyle.Render("updated: "+updated),
		tui.SummaryStyle.Render(summary))
}

// getString safely extracts a string value from a map by key.
func getString(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

// nestedString safely extracts m[key1][key2] as a string.
func nestedString(m map[string]any, key1, key2 string) string {
	if inner, ok := m[key1].(map[string]any); ok {
		return getString(inner, key2)
	}
	return ""
}

// runLastComment fetches and renders the last comment on a Jira ticket.
func runLastComment(args []string) {
	fs := flag.NewFlagSet("last-comment", flag.ExitOnError)
	if err := fs.Parse(args); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing last-comment <TICKET-KEY>")
	}
	conn := mustConnect()
	comment, err := api.FetchLastComment(conn, fs.Arg(0))
	if err != nil {
		fatal("fetching comment: %v", err)
	}
	renderLastComment(comment)
}

// runUpdate adds a comment to an existing Jira ticket via $EDITOR or stdin.
func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	fromStdin := fs.Bool("stdin", false, "Read comment from stdin instead of opening $EDITOR")
	if err := fs.Parse(args); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing update <TICKET-KEY> [-stdin]")
	}

	var text string
	var err error
	if *fromStdin {
		text, err = readAllStdin()
	} else {
		text, err = openEditor()
	}
	if err != nil {
		fatal("%v", err)
	}
	if strings.TrimSpace(text) == "" {
		fatal("comment is empty, aborting update")
	}

	key := fs.Arg(0)
	conn := mustConnect()
	if err := api.AddComment(conn, key, buildDescription(text)); err != nil {
		fatal("adding comment: %v", err)
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Comment added to"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

// openEditor writes a temp file, opens $EDITOR (or config editor), and returns the saved content.
// EDITOR may contain arguments (e.g. "code --wait") — split before exec to avoid shell injection.
func openEditor() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		editor = cfg.Editor
	}
	if editor == "" {
		return "", fmt.Errorf("EDITOR is not set; use -stdin, set the EDITOR environment variable, or add \"editor\" to ~/.config/jira-thing/jira-thing.json")
	}
	f, err := os.CreateTemp("", "jira-thing-update-*.txt")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(f.Name())
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}

	// $EDITOR is user-supplied by design (same pattern as git, kubectl).
	// strings.Fields splits args without shell involvement, preventing injection.
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], f.Name())...) // #nosec G204 G702
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor exited with error: %w", err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		return "", fmt.Errorf("reading editor output: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// readAllStdin drains stdin and returns the trimmed content.
func readAllStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// threeBusinessDaysAgo returns the date 3 weekdays before now, skipping Sat/Sun.
func threeBusinessDaysAgo(now time.Time) time.Time {
	t := now.Truncate(24 * time.Hour)
	count := 0
	for count < 3 {
		t = t.AddDate(0, 0, -1)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			count++
		}
	}
	return t
}

// runToilCheck queries Jira for toil tickets using labels from config.
func runToilCheck() {
	cfg, err := config.Load()
	if err != nil {
		fatal("%v", err)
	}
	if cfg.Project == "" || cfg.ToilMarker == "" || cfg.ToilTeam == "" {
		fatal("project, toil_marker and toil_team must be set in ~/.config/jira-thing/jira-thing.json")
	}
	conn := mustConnect()
	jql := fmt.Sprintf(
		`project = "%s" AND labels = "%s" AND labels = "%s" AND updated >= -1w`,
		cfg.Project, cfg.ToilMarker, cfg.ToilTeam,
	)
	q := api.SearchQuery{
		JQL:        jql,
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	}
	result, err := api.SearchIssues(conn, q)
	if err != nil {
		fatal("fetching toil tickets: %v", err)
	}
	if len(result.Issues) == 0 {
		fmt.Println("No toil tickets found.")
		return
	}
	fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Found %d toil ticket(s):", len(result.Issues))))
	fmt.Println()
	printTasks(result.Issues)
}

const (
	notesStart = "<!-- jira-thing:notes:start -->"
	notesEnd   = "<!-- jira-thing:notes:end -->"
)

// runToilSync queries open TOIL tickets and syncs each to a child Confluence page,
// then updates the hanger page with links to all children.
func runToilSync() {
	cfg, err := config.Load()
	if err != nil {
		fatal("%v", err)
	}
	if cfg.ConfluenceSpace == "" || cfg.TicketHanger == "" {
		fatal("confluence_space and ticket_hanger must be set in ~/.config/jira-thing/jira-thing.json")
	}
	conn := mustConnect()
	jql := fmt.Sprintf(
		`project = "%s" AND labels = "%s" AND labels = "%s" AND resolution = Unresolved ORDER BY updated DESC`,
		cfg.Project, cfg.ToilMarker, cfg.ToilTeam,
	)
	result, err := api.SearchIssues(conn, api.SearchQuery{
		JQL:        jql,
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	})
	if err != nil {
		fatal("fetching toil tickets: %v", err)
	}
	if len(result.Issues) == 0 {
		fmt.Println("No toil tickets found.")
		return
	}

	tickets := issuesToTickets(result.Issues)
	selected, err := showTableFn(tickets)
	if err != nil {
		fatal("TUI: %v", err)
	}
	if len(selected) == 0 {
		fmt.Println("No tickets selected.")
		return
	}

	selectedIssues := filterSelectedIssues(result.Issues, selected)

	hanger, err := api.FetchConfluencePage(conn, cfg.ConfluenceSpace, cfg.TicketHanger)
	if err != nil {
		fatal("%v", err)
	}
	children, err := api.ListChildPages(conn, hanger.ID)
	if err != nil {
		fatal("listing child pages: %v", err)
	}
	childMap := make(map[string]api.ConfluencePageWithBody, len(children))
	for _, c := range children {
		childMap[c.Title] = c
	}
	ticketKeys := make(map[string]bool, len(selectedIssues))
	for _, issue := range selectedIssues {
		ticketKeys[getString(issue, "key")] = true
		syncTicketPage(conn, cfg.ConfluenceSpace, hanger.ID, issue, childMap)
	}
	var manualPages []api.ConfluencePageWithBody
	for _, c := range children {
		if !ticketKeys[c.Title] {
			manualPages = append(manualPages, c)
		}
	}
	if err := api.UpdateConfluencePage(conn, hanger.ID, hanger.Version, hanger.Title, renderHangerPage(selectedIssues, manualPages)); err != nil {
		fatal("updating hanger page: %v", err)
	}
	fmt.Printf("%s %d ticket(s) to %q (%s)\n", tui.SuccessStyle.Render("Synced"), len(selectedIssues), hanger.Title, cfg.ConfluenceSpace)
}

// filterSelectedIssues returns only the issues whose keys match the TUI selection.
func filterSelectedIssues(issues []map[string]any, selected []tui.Ticket) []map[string]any {
	keys := make(map[string]bool, len(selected))
	for _, t := range selected {
		keys[t.Key] = true
	}
	filtered := make([]map[string]any, 0, len(selected))
	for _, issue := range issues {
		if keys[getString(issue, "key")] {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// syncTicketPage creates or updates a child page for one Jira issue, preserving existing notes.
func syncTicketPage(conn api.JiraConnection, spaceKey, parentID string, issue map[string]any, existing map[string]api.ConfluencePageWithBody) {
	key := getString(issue, "key")
	notes := extractNotes(existing[key].Body)
	body := renderTicketPage(issue, notes)
	if child, ok := existing[key]; ok {
		if err := api.UpdateConfluencePage(conn, child.ID, child.Version, key, body); err != nil {
			fatal("updating page %s: %v", key, err)
		}
		fmt.Printf("  %s %s\n", tui.SuccessStyle.Render("Updated:"), tui.KeyStyle.Render(key))
		return
	}
	if _, err := api.CreateConfluencePage(conn, spaceKey, key, parentID, body); err != nil {
		fatal("creating page %s: %v", key, err)
	}
	fmt.Printf("  %s %s\n", tui.SuccessStyle.Render("Created:"), tui.KeyStyle.Render(key))
}

// extractNotes returns the content between note markers in a Confluence page body.
func extractNotes(body string) string {
	start := strings.Index(body, notesStart)
	end := strings.Index(body, notesEnd)
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return body[start+len(notesStart) : end]
}

// renderTicketPage builds a Confluence storage-format page for one Jira issue.
// The details section embeds the live Jira issue via the Confluence Jira macro.
// Existing notes are preserved; empty string produces a blank notes section.
func renderTicketPage(issue map[string]any, notes string) string {
	key := html.EscapeString(getString(issue, "key"))
	if notes == "" {
		today := time.Now().Format("2006-01-02")
		notes = fmt.Sprintf(
			`<p><time datetime="%s" /></p>`+
				`<ac:adf-extension><ac:adf-node type="panel"><ac:adf-attribute key="panelType">note</ac:adf-attribute><ac:adf-content><p> </p></ac:adf-content></ac:adf-node></ac:adf-extension>`,
			today,
		)
	}
	return fmt.Sprintf(
		`<!-- jira-thing:details:start --><h2>Jira Details</h2>`+
			`<ac:structured-macro ac:name="jira" ac:schema-version="1">`+
			`<ac:parameter ac:name="key">%s</ac:parameter>`+
			`</ac:structured-macro>`+
			`<!-- jira-thing:details:end -->`+
			`<h2>Notes</h2>%s%s%s`,
		key, notesStart, notes, notesEnd,
	)
}

// renderHangerPage builds a Confluence storage-format page listing tickets and any manually
// created child pages as links.
func renderHangerPage(issues []map[string]any, manualPages []api.ConfluencePageWithBody) string {
	if len(issues) == 0 && len(manualPages) == 0 {
		return fmt.Sprintf("<p>No open TOIL tickets as of %s.</p>", time.Now().Format("2006-01-02"))
	}
	var sb strings.Builder
	sb.WriteString("<ul>")
	for _, issue := range issues {
		key := html.EscapeString(getString(issue, "key"))
		f, _ := issue["fields"].(map[string]any)
		summary := ""
		if f != nil {
			summary = html.EscapeString(getString(f, "summary"))
		}
		fmt.Fprintf(&sb,
			`<li><ac:link><ri:page ri:content-title=%q/><ac:plain-text-link-body><![CDATA[%s - %s]]></ac:plain-text-link-body></ac:link></li>`,
			key, key, summary)
	}
	for _, p := range manualPages {
		title := html.EscapeString(p.Title)
		fmt.Fprintf(&sb,
			`<li><ac:link><ri:page ri:content-title=%q/><ac:plain-text-link-body><![CDATA[%s]]></ac:plain-text-link-body></ac:link></li>`,
			title, title)
	}
	sb.WriteString("</ul>")
	return sb.String()
}
