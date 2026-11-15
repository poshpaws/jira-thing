package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

// createFlags holds runCreate's parsed flags.
type createFlags struct {
	templatePath    string
	batchPath       string
	descriptionFile string
	project         string
	summary         string
	issueType       string
	priority        string
	labels          string
	components      string
	assignee        string
	epic            string
	nonInteractive  bool
	dryRun          bool
	json            bool
	dedupe          bool
}

// parseCreateFlags parses runCreate's flag set, exiting on a parse error.
func parseCreateFlags(args []string) createFlags {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	templatePath := fs.String("t", "", "Path to template file")
	batchPath := fs.String("f", "", "Path to a JSON array of ticket templates for batch create")
	descriptionFile := fs.String("description-file", "", "Path to a markdown file for the description (overrides template/prompt)")
	project := fs.String("project", "", "Project key or ID (e.g. PROJ) — overrides the template; with no template on the search path, create from flags alone")
	summary := fs.String("summary", "", "Ticket summary — overrides the template, skipping the prompt")
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
	dedupe := fs.Bool("dedupe-by-summary", false, "Skip creating if an open ticket in the project has the same summary")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	return createFlags{
		*templatePath, *batchPath, *descriptionFile, *project, *summary, *issueType, *priority,
		*labels, *components, *assignee, *epic, *nonInteractive, *dryRun, *jsonOut, *dedupe,
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
// reports how many succeeded. -dedupe-by-summary skips any ticket that would
// duplicate an open one.
func runCreate(args []string) {
	flags := parseCreateFlags(args)
	if flags.batchPath != "" {
		runBatchCreate(flags.batchPath, flags.dryRun, flags.dedupe)
		return
	}

	tmpl := loadCreateTemplate(flags.templatePath, flags.project != "")
	if flags.summary != "" {
		tmpl["summary"] = flags.summary
	}
	if err := resolveCreateFields(tmpl, flags.descriptionFile, flags.nonInteractive); err != nil {
		fatal("%v", err)
	}

	conn := mustConnect()
	opts := ops.CreateOptions{Dedupe: ops.NewDedupeGuard(conn, flags.dedupe), DryRun: flags.dryRun}
	res, err := ops.CreateTicket(conn, createRequest(tmpl, flags), opts)
	if err != nil {
		fatal("%v", err)
	}
	reportCreateResult(conn, res, flags.json)
}

// createRequest pairs the resolved template with the first-class field
// flags, each of which overrides whatever the template holds.
func createRequest(tmpl map[string]any, flags createFlags) ops.CreateRequest {
	return ops.CreateRequest{
		Fields:     tmpl,
		Project:    flags.project,
		IssueType:  flags.issueType,
		Priority:   flags.priority,
		Labels:     template.SplitLabels(flags.labels),
		Components: template.SplitLabels(flags.components),
		Assignee:   flags.assignee,
		Parent:     flags.epic,
	}
}

// reportCreateResult prints what the create did: the duplicate that made it
// unnecessary, the dry-run payload, or the new ticket.
func reportCreateResult(conn api.JiraConnection, res ops.CreateResult, jsonOut bool) {
	switch res.Status {
	case ops.StatusExists:
		reportDuplicate(res, jsonOut)
	case ops.StatusDryRun:
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint, res.Payload)
		printParentLinkStep(res.Parent)
	default:
		if err := parentLinkError(res); err != nil {
			fatal("%v", err)
		}
		reportCreated(res, jsonOut)
	}
}

// parentLinkError reports a ticket that was created but not linked to its parent.
func parentLinkError(res ops.CreateResult) error {
	if res.ParentErr == nil {
		return nil
	}
	return fmt.Errorf("created %s but failed to link to parent %s: %w", res.Key, res.Parent, res.ParentErr)
}

// reportCreated prints the newly created ticket's key and URL, as JSON when
// jsonOut is set, otherwise as styled text.
func reportCreated(res ops.CreateResult, jsonOut bool) {
	if jsonOut {
		printJSONResult(jsonResult{Key: res.Key, URL: res.URL, Status: res.Status})
		return
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Created ticket:"), tui.KeyStyle.Render(res.Key))
	fmt.Printf("URL: %s\n", res.URL)
}

// loadCreateTemplate loads the template at path, exiting with a pointer to
// the search path (and how to specify one directly) when none was found.
// With allowNone (a -project flag was given) and no path, a missing default
// template yields an empty one, so a ticket can be created from flags alone.
func loadCreateTemplate(path string, allowNone bool) map[string]any {
	tmpl, err := template.Load(path)
	if path == "" && allowNone && errors.Is(err, template.ErrTemplateNotFound) {
		return map[string]any{}
	}
	if err != nil {
		if path == "" {
			fatal("no template found in search path.\n\nCreate one first:\n  jira-thing template <TICKET-KEY> [-o ticket_template.json]\n\nThen place ticket_template.json in one of:\n  - current directory\n  - same directory as the jira-thing binary\n  - $XDG_CONFIG_HOME/jira-thing/\n  - ~/.config/jira-thing/\n\nOr specify a path directly:\n  jira-thing create -t /path/to/template.json")
		}
		fatal("loading template: %v", err)
	}
	return tmpl
}

// printParentLinkStep notes the epic/parent link a real create makes right
// after the POST, which the create payload itself doesn't contain.
func printParentLinkStep(parentKey string) {
	if parentKey == "" {
		return
	}
	fmt.Printf("\nThen: link the new ticket to %s (sets \"parent\", falling back to the classic \"Epic Link\" field)\n", parentKey)
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
	return adf.FromMarkdown(text)
}
