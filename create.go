package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

// createFlags holds runCreate's parsed flags.
type createFlags struct {
	templatePath    string
	batchPath       string
	descriptionFile string
	issueType       string
	priority        string
	labels          string
	components      string
	assignee        string
	epic            string
	nonInteractive  bool
	dryRun          bool
	json            bool
}

// parseCreateFlags parses runCreate's flag set, exiting on a parse error.
func parseCreateFlags(args []string) createFlags {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	templatePath := fs.String("t", "", "Path to template file")
	batchPath := fs.String("f", "", "Path to a JSON array of ticket templates for batch create")
	descriptionFile := fs.String("description-file", "", "Path to a markdown file for the description (overrides template/prompt)")
	issueType := fs.String("type", "", "Issue type name (e.g. Task, Bug, Story) — overrides the template")
	priority := fs.String("priority", "", "Priority name (e.g. High, Medium, Low) — overrides the template")
	labels := fs.String("labels", "", "Comma-separated labels — overrides the template")
	components := fs.String("components", "", "Comma-separated component names — overrides the template")
	assignee := fs.String("assignee", "", `Who to assign: "me", "unassign", or a Jira account ID`)
	epic := fs.String("epic", "", "Epic/parent ticket key to link this ticket to — overrides the template")
	nonInteractive := fs.Bool("non-interactive", false, "Fail instead of prompting for missing required fields")
	fs.BoolVar(nonInteractive, "y", false, "Alias for -non-interactive")
	dryRun := fs.Bool("dry-run", false, "Show the resolved payload without creating anything")
	jsonOut := fs.Bool("json", false, "Print {key, url, status} as JSON instead of styled text")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	return createFlags{
		*templatePath, *batchPath, *descriptionFile, *issueType, *priority,
		*labels, *components, *assignee, *epic, *nonInteractive, *dryRun, *jsonOut,
	}
}

// runCreate loads a template (or a batch file of templates) and creates one
// or more Jira tickets. With -t, summary/description/parent already present
// in the template are used as-is; missing fields are prompted for unless
// -non-interactive/-y is set, in which case a missing summary is a fatal
// error. First-class flags (-type, -priority, -labels, -components,
// -assignee, -epic) always override whatever the template holds, so a ticket
// can be created end-to-end from flags with no template at all. With -f,
// every entry in the JSON array is created non-interactively and the command
// reports how many succeeded.
func runCreate(args []string) {
	flags := parseCreateFlags(args)
	if flags.batchPath != "" {
		runBatchCreate(flags.batchPath, flags.dryRun)
		return
	}

	tmpl := loadCreateTemplate(flags.templatePath)
	if err := resolveCreateFields(tmpl, flags.descriptionFile, flags.nonInteractive); err != nil {
		fatal("%v", err)
	}

	conn := mustConnect()
	if err := applyCreateFieldFlags(conn, tmpl, flags); err != nil {
		fatal("%v", err)
	}

	if flags.dryRun {
		printCreateDryRun(conn, tmpl)
		return
	}

	key, err := createTicket(conn, tmpl)
	if err != nil {
		fatal("%v", err)
	}
	reportCreated(conn, key, flags.json)
}

// reportCreated prints the newly created ticket's key and URL, as JSON when
// jsonOut is set, otherwise as styled text.
func reportCreated(conn api.JiraConnection, key string, jsonOut bool) {
	url := conn.BaseURL + "/browse/" + key
	if jsonOut {
		printJSONResult(jsonResult{Key: key, URL: url, Status: "created"})
		return
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Created ticket:"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s\n", url)
}

// applyCreateFieldFlags overlays first-class create flags onto tmpl, each
// overriding any value the template already holds. Only flags actually given
// produce a change. Must run after resolveCreateFields (so it overrides, not
// competes with, prompted/template values) and before prepareTicketPayload
// (so -epic's parent key and -assignee's resolved value are still present
// for it to consume).
func applyCreateFieldFlags(conn api.JiraConnection, tmpl map[string]any, flags createFlags) error {
	if flags.issueType != "" {
		tmpl["issuetype"] = map[string]any{"name": flags.issueType}
	}
	if flags.priority != "" {
		tmpl["priority"] = map[string]any{"name": flags.priority}
	}
	if flags.labels != "" {
		tmpl["labels"] = template.SplitLabels(flags.labels)
	}
	if flags.components != "" {
		tmpl["components"] = buildComponents(flags.components)
	}
	if flags.epic != "" {
		tmpl["parent"] = flags.epic
	}
	if flags.assignee == "" {
		return nil
	}
	resolved, err := template.ResolveAssigneeArg(conn, flags.assignee)
	if err != nil {
		return err
	}
	tmpl["assignee"] = resolved
	return nil
}

// buildComponents turns a comma-separated component name list into the Jira
// API's expected component object shape.
func buildComponents(names string) []map[string]any {
	parts := template.SplitLabels(names)
	components := make([]map[string]any, len(parts))
	for i, name := range parts {
		components[i] = map[string]any{"name": name}
	}
	return components
}

// loadCreateTemplate loads the template at path, exiting with a pointer to
// the search path (and how to specify one directly) when none was found.
func loadCreateTemplate(path string) map[string]any {
	tmpl, err := template.Load(path)
	if err != nil {
		if path == "" {
			fatal("no template found in search path.\n\nCreate one first:\n  jira-thing template <TICKET-KEY> [-o ticket_template.json]\n\nThen place ticket_template.json in one of:\n  - current directory\n  - same directory as the jira-thing binary\n  - $XDG_CONFIG_HOME/jira-thing/\n  - ~/.config/jira-thing/\n\nOr specify a path directly:\n  jira-thing create -t /path/to/template.json")
		}
		fatal("loading template: %v", err)
	}
	return tmpl
}

// printCreateDryRun resolves the ticket payload and prints what would be
// POSTed, without creating anything.
func printCreateDryRun(conn api.JiraConnection, tmpl map[string]any) {
	payload, _, err := prepareTicketPayload(conn, tmpl)
	if err != nil {
		fatal("%v", err)
	}
	printDryRun("POST", conn.BaseURL+api.IssueEndpoint, payload)
}

// resolveCreateFields fills in summary and description on tmpl: an explicit
// -description-file always wins for the description; otherwise an existing
// template value is used; anything still missing is prompted for unless
// nonInteractive is set.
func resolveCreateFields(tmpl map[string]any, descriptionFile string, nonInteractive bool) error {
	if descriptionFile != "" {
		data, err := os.ReadFile(descriptionFile) // #nosec G304 -- user-supplied CLI input
		if err != nil {
			return fmt.Errorf("reading description file: %w", err)
		}
		tmpl["description"] = string(data)
	}

	summary, err := resolveSummary(tmpl, nonInteractive)
	if err != nil {
		return err
	}
	description := resolveDescription(tmpl, nonInteractive)

	tmpl["summary"] = summary
	if description != "" {
		tmpl["description"] = buildDescription(description)
	} else {
		delete(tmpl, "description")
	}
	return nil
}

// resolveSummary returns the template's summary if already set, otherwise
// prompts for one; in non-interactive mode a missing summary is a fatal error.
func resolveSummary(tmpl map[string]any, nonInteractive bool) (string, error) {
	if s, ok := tmpl["summary"].(string); ok && strings.TrimSpace(s) != "" {
		return s, nil
	}
	if nonInteractive {
		return "", fmt.Errorf(`summary is required: set "summary" in the template, or omit -non-interactive to be prompted`)
	}
	summary := promptLine("Enter ticket summary: ")
	if summary == "" {
		return "", fmt.Errorf("summary is required")
	}
	return summary, nil
}

// resolveDescription returns the template's description if already set,
// otherwise prompts for one. Description is optional, so a non-interactive
// run with none given simply creates the ticket without one.
func resolveDescription(tmpl map[string]any, nonInteractive bool) string {
	if d, ok := tmpl["description"].(string); ok && d != "" {
		return d
	}
	if nonInteractive {
		return ""
	}
	return promptLine("Enter ticket description: ")
}

// prepareTicketPayload strips non-updatable fields, resolves a self-assignee
// marker, and extracts the optional "parent" ticket key, returning the
// request body CreateIssue would send and the parent key to link afterwards.
func prepareTicketPayload(conn api.JiraConnection, tmpl map[string]any) (map[string]any, string, error) {
	parentKey, _ := tmpl["parent"].(string)
	delete(tmpl, "parent")
	template.StripExcludedFields(tmpl)

	if tmpl["assignee"] == template.AssigneeSelf {
		me, err := api.FetchMyself(conn)
		if err != nil {
			return nil, "", fmt.Errorf("fetching current user: %w", err)
		}
		template.ResolveAssignee(tmpl, me["accountId"].(string))
	}
	return tmpl, parentKey, nil
}

// createTicket prepares the ticket payload, creates the issue, links it to
// the parent if one was given, and returns the new ticket's key.
func createTicket(conn api.JiraConnection, tmpl map[string]any) (string, error) {
	payload, parentKey, err := prepareTicketPayload(conn, tmpl)
	if err != nil {
		return "", err
	}

	result, err := api.CreateIssue(conn, payload)
	if err != nil {
		return "", fmt.Errorf("creating issue: %w", err)
	}
	key := getString(result, "key")

	if parentKey != "" {
		if err := api.AddIssuesToEpic(conn, parentKey, []string{key}); err != nil {
			return key, fmt.Errorf("created %s but failed to link to parent %s: %w", key, parentKey, err)
		}
	}
	return key, nil
}

// runBatchCreate reads a JSON array of ticket templates from path and creates
// each one non-interactively, continuing past per-ticket failures and
// reporting how many of the total succeeded. With dryRun, resolves and prints
// each ticket's payload instead of creating anything.
func runBatchCreate(path string, dryRun bool) {
	tickets := loadBatchTickets(path)
	conn := mustConnect()
	if dryRun {
		previewBatchTickets(conn, tickets)
		return
	}

	fmt.Printf("Creating %d ticket(s) from %s...\n", len(tickets), path)
	created := 0
	for i, tmpl := range tickets {
		key, err := createBatchTicket(conn, tmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
			continue
		}
		fmt.Printf("  %s %s  %s\n", tui.SuccessStyle.Render("Created:"), tui.KeyStyle.Render(key), getString(tmpl, "summary"))
		created++
	}
	fmt.Printf("\n%s %d of %d ticket(s) created\n", tui.SuccessStyle.Render("Done:"), created, len(tickets))
}

// loadBatchTickets reads and parses path as a non-empty JSON array of ticket templates.
func loadBatchTickets(path string) []map[string]any {
	data, err := os.ReadFile(path) // #nosec G304 -- user-supplied CLI input
	if err != nil {
		fatal("reading batch file: %v", err)
	}
	var tickets []map[string]any
	if err := json.Unmarshal(data, &tickets); err != nil {
		fatal("parsing batch file as a JSON array: %v", err)
	}
	if len(tickets) == 0 {
		fatal("no tickets found in %s", path)
	}
	return tickets
}

// previewBatchTickets resolves and prints each batch entry's payload without
// creating anything, continuing past per-ticket resolution failures just like
// a real batch create would continue past per-ticket API failures.
func previewBatchTickets(conn api.JiraConnection, tickets []map[string]any) {
	fmt.Println(tui.HeadingStyle.Render(fmt.Sprintf("Dry run — %d ticket(s), none will be created:", len(tickets))))
	for i, tmpl := range tickets {
		if err := resolveCreateFields(tmpl, "", true); err != nil {
			fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
			continue
		}
		payload, _, err := prepareTicketPayload(conn, tmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s ticket %d: %v\n", tui.ErrorStyle.Render("Error:"), i+1, err)
			continue
		}
		fmt.Printf("\n[%d/%d]\n", i+1, len(tickets))
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint, payload)
	}
}

// createBatchTicket resolves fields non-interactively and creates a single
// ticket from a batch entry.
func createBatchTicket(conn api.JiraConnection, tmpl map[string]any) (string, error) {
	if err := resolveCreateFields(tmpl, "", true); err != nil {
		return "", err
	}
	return createTicket(conn, tmpl)
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

// buildDescription converts markdown text to the Jira Atlassian Document Format structure.
func buildDescription(text string) map[string]any {
	return markdownToADF(text)
}
