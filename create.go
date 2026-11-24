package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/adf"
	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/termsafe"
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
	custom          customFlag
	fixVersions     stringList
	followUp        createFollowUpFlags
	named           string
	noTemplate      bool
}

// createFollowUpFlags are the create flags applied after the ticket exists.
type createFollowUpFlags struct {
	sprint *string
	board  *int
	points *string
}

// addCreateFollowUpFlags defines -sprint, -board and -points on fs.
func addCreateFollowUpFlags(fs *flag.FlagSet) createFollowUpFlags {
	return createFollowUpFlags{
		sprint: fs.String("sprint", "", `Sprint to add the new ticket to: "current", "next" or a sprint ID`),
		board:  fs.Int("board", 0, `Board -sprint is looked up on (default: "board" in config for its project, else the project's only sprint board)`),
		points: fs.String("points", "", `Story points, e.g. 3 or 0.5, set after the create (field: "story_points_field" in config, else the usual defaults)`),
	}
}

// parseCreateFlags parses runCreate's flag set, exiting on a parse error.
func parseCreateFlags(args []string) createFlags {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	templatePath := fs.String("t", "", "Path to template file")
	noTemplate := fs.Bool("no-template", false, `Ignore templates on the search path and start from flags alone (see also -T standard)`)
	named := fs.String("T", "", `Named template to start from (see "template list"): a file in templates_dir, or the built-in "standard" or "toil"`)
	batchPath := fs.String("f", "", "Path to a JSON array of ticket templates for batch create")
	descriptionFile := fs.String("description-file", "", `Path to a markdown file for the description, or "-" for stdin (overrides template/prompt; stdin implies -y)`)
	project := fs.String("project", "", "Project key or ID (e.g. PROJ) — overrides the template; with no template on the search path, create from flags alone")
	summary := fs.String("summary", "", "Ticket summary — overrides the template, skipping the prompt")
	issueType := fs.String("type", "", "Issue type name (e.g. Task, Bug, Story) — overrides the template")
	priority := fs.String("priority", "", "Priority name (e.g. High, Medium, Low) — overrides the template")
	labels := fs.String("labels", "", "Comma-separated labels — overrides the template")
	components := fs.String("components", "", "Comma-separated component names — overrides the template")
	assignee := fs.String("assignee", "", `Who to assign: "me", "unassign", a name, an email, or a Jira account ID`)
	epic := fs.String("epic", "", "Epic/parent ticket key to link this ticket to — overrides the template")
	nonInteractive := fs.Bool("non-interactive", false, "Fail instead of prompting for missing required fields")
	fs.BoolVar(nonInteractive, "y", false, "Alias for -non-interactive")
	dryRun := fs.Bool("dry-run", false, "Show the resolved payload without creating anything")
	jsonOut := fs.Bool("json", false, "Print {key, url, status} as JSON instead of styled text")
	dedupe := fs.Bool("dedupe-by-summary", false, "Skip creating if an open ticket in the project has the same summary")
	var custom customFlag
	fs.Var(&custom, "custom", customHelp)
	var fixVersions stringList
	fs.Var(&fixVersions, "fix-version", "Fix version name (repeatable, or comma-separated)")
	followUp := addCreateFollowUpFlags(fs)
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}
	return createFlags{
		*templatePath, *batchPath, *descriptionFile, *project, *summary, *issueType, *priority,
		*labels, *components, *assignee, *epic, *nonInteractive, *dryRun, *jsonOut, *dedupe, custom, fixVersions, followUp, *named, *noTemplate,
	}
}

// runCreate loads a template (or a batch file of templates) and creates one
// or more Jira tickets. With -t, summary/description/parent already present
// in the template are used as-is; missing fields are prompted for unless
// -non-interactive/-y is set, in which case a missing summary is a fatal
// error. First-class flags (-type, -priority, -labels, -components,
// -assignee, -epic, -sprint, -points) always override whatever the template holds, so a ticket
// can be created end-to-end from flags with no template at all. With -f,
// every entry in the JSON array is created non-interactively and the command
// reports how many succeeded. -dedupe-by-summary skips any ticket that would
// duplicate an open one.
func runCreate(args []string) {
	flags := parseCreateFlags(args)
	cfg, err := config.Load()
	if err != nil {
		fatal("loading config: %v", err)
	}
	if flags.batchPath != "" {
		batch := ops.ConfigBatchOptions(cfg)
		batch.Dedupe, batch.DryRun = flags.dedupe, flags.dryRun
		runBatchCreate(flags.batchPath, batch)
		return
	}

	tmpl := createBase(flags, cfg)
	if flags.summary != "" {
		tmpl["summary"] = flags.summary
	}
	if err := resolveCreateFields(tmpl, flags.descriptionFile, flags.nonInteractive); err != nil {
		fatal("%v", err)
	}

	conn := mustConnect()
	opts := ops.CreateOptions{Dedupe: ops.NewDedupeGuard(conn, flags.dedupe), DryRun: flags.dryRun}
	req := createRequest(tmpl, flags)
	req.Defaults = ops.DefaultsFromConfig(cfg)
	addCreateFollowUps(&req, flags, cfg)
	res, err := ops.CreateTicket(conn, req, opts)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
	}
	reportCreateResult(conn, res, flags.json)
}

// createRequest pairs the resolved template with the first-class field
// flags, each of which overrides whatever the template holds.
func createRequest(tmpl map[string]any, flags createFlags) ops.CreateRequest {
	return ops.CreateRequest{
		Fields:      tmpl,
		Project:     flags.project,
		IssueType:   flags.issueType,
		Priority:    flags.priority,
		Labels:      template.SplitLabels(flags.labels),
		Components:  template.SplitLabels(flags.components),
		Assignee:    flags.assignee,
		Parent:      flags.epic,
		Custom:      []ops.CustomValue(flags.custom),
		FixVersions: template.SplitLabels(strings.Join(flags.fixVersions, ",")),
	}
}

// addCreateFollowUps adds -sprint and -points to req, exiting on a
// non-numeric -points.
func addCreateFollowUps(req *ops.CreateRequest, flags createFlags, cfg config.Config) {
	req.StoryPoints = parsePoints(*flags.followUp.points)
	if req.StoryPoints != nil {
		req.PointFields = ops.StoryPointFields(cfg)
	}
	req.Sprint = *flags.followUp.sprint
	if req.Sprint == "" {
		return
	}
	project := flags.project
	if project == "" {
		project = ops.FieldsProject(req.Fields)
	}
	req.SprintScope = ops.CreateSprintScope(cfg, *flags.followUp.board, project)
}

// reportCreateResult prints what the create did: the duplicate that made it
// unnecessary, the dry-run payload, or the new ticket.
func reportCreateResult(conn api.JiraConnection, res ops.CreateResult, jsonOut bool) {
	switch res.Status {
	case ops.StatusExists:
		reportDuplicate(res, jsonOut)
	case ops.StatusDryRun:
		printDryRun("POST", conn.BaseURL+api.IssueEndpoint, res.Payload)
		printFollowUpSteps(res)
	default:
		if err := ops.FollowUpError(res); err != nil {
			fatal("%s", termsafe.Clean(err.Error()))
		}
		reportCreated(res, jsonOut)
	}
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
	if res.Sprint.ID != 0 {
		fmt.Printf("Sprint: %s\n", ops.SprintLabel(res.Sprint))
	}
	if res.PointsField != "" {
		fmt.Printf("Story points: %v (%s)\n", *res.StoryPoints, res.PointsField)
	}
}

// printFollowUpSteps notes what a real create does right after the POST,
// which the create payload itself doesn't contain: the epic/parent link,
// the sprint and the story points.
func printFollowUpSteps(res ops.CreateResult) {
	if res.Parent != "" {
		fmt.Printf("\nThen: link the new ticket to %s (sets \"parent\", falling back to the classic \"Epic Link\" field)\n", res.Parent)
	}
	if res.Sprint.ID != 0 {
		fmt.Printf("Then: add the new ticket to sprint %s\n", ops.SprintLabel(res.Sprint))
	}
	if res.StoryPoints != nil {
		fmt.Printf("Then: set story points to %v on the new ticket's edit screen\n", *res.StoryPoints)
	}
}

// promptAssigneeChoice asks the user whether to use self or the original assignee in the template.
// If no assignee is present, it defaults to self without prompting.
func promptAssigneeChoice(tmpl map[string]any) {
	assignee, ok := tmpl["assignee"].(map[string]any)
	if !ok {
		ops.SetTemplateAssignee(tmpl, false)
		return
	}
	displayName := getString(assignee, "displayName")
	if displayName == "" {
		displayName = "original user"
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Assignee: (1) current user (self) or (2) %s? [1/2]: ", displayName)
	line, _ := reader.ReadString('\n')
	keep := strings.TrimSpace(line) == "2"
	ops.SetTemplateAssignee(tmpl, keep)
	if keep {
		fmt.Printf("Keeping assignee: %s\n", displayName)
		return
	}
	fmt.Println("Assignee set to current user (resolved at ticket creation time)")
}

// buildDescription converts markdown text to the Jira Atlassian Document Format structure.
func buildDescription(text string) map[string]any {
	return adf.FromMarkdown(text)
}
