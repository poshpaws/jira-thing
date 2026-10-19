package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/tui"
)

// stringSlice collects repeated occurrences of a flag (e.g. -f a -f b) into a slice.
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

const attachUsage = "usage: jira-thing attach <TICKET-KEY> [<file-path> | -f file ...] | [--stdin --name x.md]"

// reorderFlagsBeforeKey moves flags (and their values) ahead of positional
// arguments so flag.Parse doesn't stop at the first non-flag argument when
// the ticket key is given before its flags (e.g. "attach PROJ-1 -f a.txt").
func reorderFlagsBeforeKey(args []string) []string {
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
	return append(reordered, positional...)
}

// runAttach uploads one or more local files, or piped stdin content under an
// explicit name, as attachments on an existing Jira ticket.
func runAttach(args []string) {
	fs := flag.NewFlagSet("attach", flag.ExitOnError)
	var files stringSlice
	fs.Var(&files, "f", "File to attach (repeatable: -f a -f b)")
	stdin := fs.Bool("stdin", false, "Read attachment content from stdin")
	name := fs.String("name", "", "Attachment filename when using -stdin")
	if err := fs.Parse(reorderFlagsBeforeKey(args)); err != nil || fs.NArg() < 1 {
		fatal(attachUsage)
	}
	key := fs.Arg(0)
	if fs.NArg() > 1 {
		files = append(files, fs.Arg(1)) // legacy "attach KEY file-path" form
	}

	conn := mustConnect()
	switch {
	case *stdin:
		attachStdin(conn, key, *name)
	case len(files) > 0:
		attachFiles(conn, key, files)
	default:
		fatal(attachUsage)
	}
}

// attachFiles uploads each local file in turn, failing fast on the first error.
func attachFiles(conn api.JiraConnection, key string, files []string) {
	for _, path := range files {
		if _, err := api.AddAttachment(conn, key, path); err != nil {
			fatal("attaching %s: %v", path, err)
		}
		fmt.Printf("%s %s %s %s\n",
			tui.SuccessStyle.Render("Attached"), path, tui.SuccessStyle.Render("to"), tui.KeyStyle.Render(key))
	}
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

// attachStdin uploads the contents of stdin as an attachment named name.
func attachStdin(conn api.JiraConnection, key, name string) {
	if name == "" {
		fatal("-name is required with -stdin")
	}
	if _, err := api.AddAttachmentReader(conn, key, name, os.Stdin); err != nil {
		fatal("attaching %s: %v", name, err)
	}
	fmt.Printf("%s %s %s %s\n",
		tui.SuccessStyle.Render("Attached"), name, tui.SuccessStyle.Render("to"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}
