package main

import (
	"flag"
	"fmt"

	"jira-thing/internal/api"
	mcpsrv "jira-thing/internal/mcpserver"
	"jira-thing/internal/tui"
)

const (
	linkUsage = "usage: jira-thing link <OUTWARD-KEY> <INWARD-KEY> -type <name> [-dry-run] [-json]\n" +
		"  the link type's outward text reads OUTWARD-KEY -> INWARD-KEY, e.g.\n" +
		"  \"jira-thing link PROJ-100 PROJ-200 -type Blocks\" means PROJ-100 blocks PROJ-200\n" +
		"  run \"jira-thing link-types\" to see the types on this instance"
	unlinkUsage     = "usage: jira-thing unlink <KEY> <OTHER-KEY>"
	linkTypesUsage  = "usage: jira-thing link-types [-json]"
	remoteLinkUsage = "usage: jira-thing remote-link <KEY> -url <url> [-title <title>]"
)

// linkFlags holds runLink's parsed flags and positional arguments.
type linkFlags struct {
	outward  string
	inward   string
	linkType string
	dryRun   bool
	jsonOut  bool
}

// linkJSON is the machine-readable shape "link -json" prints.
type linkJSON struct {
	Outward string `json:"outward"`
	Inward  string `json:"inward"`
	Type    string `json:"type"`
	Status  string `json:"status"`
}

// parseLinkFlags parses runLink's flag set, exiting on a parse error.
func parseLinkFlags(args []string) linkFlags {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	linkType := fs.String("type", "", `Link type name (e.g. "Blocks", "Relates"); see link-types`)
	dryRun := fs.Bool("dry-run", false, "Show the link request without creating it")
	jsonOut := fs.Bool("json", false, "Print {outward, inward, type, status} as JSON instead of styled text")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 2 {
		fatal(linkUsage)
	}
	return linkFlags{fs.Arg(0), fs.Arg(1), *linkType, *dryRun, *jsonOut}
}

// runLinkTypes lists the issue link types configured on the Jira instance.
func runLinkTypes(args []string) {
	fs := flag.NewFlagSet("link-types", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Print the link types as a JSON array")
	if err := fs.Parse(args); err != nil {
		fatal(linkTypesUsage)
	}
	types := mustFetchLinkTypes(mustConnect())
	if *jsonOut {
		printJSONLine(types)
		return
	}
	fmt.Print(mcpsrv.FormatLinkTypes(types))
}

// runLink creates an issue link non-interactively. The type is validated
// against the instance's live link types; a missing or unknown -type exits
// with that list rather than an opaque API error.
func runLink(args []string) {
	flags := parseLinkFlags(args)
	conn := mustConnect()
	types := mustFetchLinkTypes(conn)
	if flags.linkType == "" {
		fatal("-type is required\n\n%s", mcpsrv.FormatLinkTypes(types))
	}
	linkType, err := mcpsrv.FindLinkType(types, flags.linkType)
	if err != nil {
		fatal("%v\n\n%s", err, mcpsrv.FormatLinkTypes(types))
	}
	if flags.dryRun {
		printDryRun("POST", conn.BaseURL+api.IssueLinkEndpoint, map[string]any{
			"type":         map[string]any{"name": linkType.Name},
			"outwardIssue": map[string]any{"key": flags.outward},
			"inwardIssue":  map[string]any{"key": flags.inward},
		})
		return
	}
	if err := api.LinkIssues(conn, flags.outward, flags.inward, linkType.Name); err != nil {
		fatal("linking %s to %s: %v", flags.outward, flags.inward, err)
	}
	reportLinked(flags, linkType)
}

// reportLinked prints the created link, spelling out its direction.
func reportLinked(flags linkFlags, linkType api.IssueLinkType) {
	if flags.jsonOut {
		printJSONLine(linkJSON{Outward: flags.outward, Inward: flags.inward, Type: linkType.Name, Status: "linked"})
		return
	}
	fmt.Printf("%s %s %s %s (%s)\n", tui.SuccessStyle.Render("Linked"),
		tui.KeyStyle.Render(flags.outward), linkType.Outward, tui.KeyStyle.Render(flags.inward), linkType.Name)
}

// runUnlink removes the link between two issues, whichever type connects them.
func runUnlink(args []string) {
	if len(args) < 2 {
		fatal(unlinkUsage)
	}
	key, other := args[0], args[1]
	if err := api.UnlinkIssues(mustConnect(), key, other); err != nil {
		fatal("unlinking %s from %s: %v", key, other, err)
	}
	fmt.Printf("%s %s from %s\n", tui.SuccessStyle.Render("Unlinked"), tui.KeyStyle.Render(key), tui.KeyStyle.Render(other))
}

// runRemoteLink attaches a remote web link to an issue; the title defaults to the URL.
func runRemoteLink(args []string) {
	fs := flag.NewFlagSet("remote-link", flag.ExitOnError)
	linkURL := fs.String("url", "", "The URL to link (required)")
	title := fs.String("title", "", "Display text for the link (default: the URL)")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 1 || *linkURL == "" {
		fatal(remoteLinkUsage)
	}
	if *title == "" {
		*title = *linkURL
	}
	key := fs.Arg(0)
	if err := api.AddRemoteLink(mustConnect(), key, *linkURL, *title); err != nil {
		fatal("adding remote link to %s: %v", key, err)
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Added remote link to"), tui.KeyStyle.Render(key))
}

// mustFetchLinkTypes reads the instance's live link types, exiting on failure.
func mustFetchLinkTypes(conn api.JiraConnection) []api.IssueLinkType {
	types, err := api.FetchIssueLinkTypes(conn)
	if err != nil {
		fatal("fetching link types: %v", err)
	}
	return types
}
