package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

const deleteUsage = "usage: jira-thing delete <TICKET-KEY> [-cascade] [-y] [-dry-run] [-json]"

// errConfirmNeeded means a delete ran without a terminal to confirm in and without -y.
var errConfirmNeeded = errors.New("refusing to delete without confirmation: pass -y, or run in a terminal")

// deleteJSON is the -json output of delete.
type deleteJSON struct {
	Key      string   `json:"key"`
	Summary  string   `json:"summary"`
	Subtasks []string `json:"subtasks"`
	Status   string   `json:"status"`
}

// deleteFlags holds runDelete's parsed flags and positional argument.
type deleteFlags struct {
	req    ops.DeleteRequest
	yes    bool
	dryRun bool
	json   bool
}

// parseDeleteFlags parses runDelete's flag set, exiting on a parse error.
func parseDeleteFlags(args []string) deleteFlags {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	cascade := fs.Bool("cascade", false, "Also delete the ticket's subtasks")
	yes := fs.Bool("y", false, "Don't ask for confirmation")
	dryRun := fs.Bool("dry-run", false, "Show what would be deleted, deleting nothing")
	jsonOut := fs.Bool("json", false, "Print {key, summary, subtasks, status} as JSON")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() != 1 {
		fatal(deleteUsage)
	}
	return deleteFlags{req: ops.DeleteRequest{Key: fs.Arg(0), Cascade: *cascade}, yes: *yes, dryRun: *dryRun, json: *jsonOut}
}

// runDelete permanently deletes a ticket after showing it and asking to
// confirm, unless -y. Without a terminal, -y is required.
func runDelete(args []string) {
	f := parseDeleteFlags(args)
	conn := mustConnect()
	if !f.dryRun && !f.yes {
		if err := confirmDelete(conn, f.req); err != nil {
			fatal("%s", termsafe.Clean(err.Error()))
			return
		}
	}
	res, err := ops.DeleteTicket(conn, f.req, f.dryRun)
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
		return
	}
	reportDelete(res, f.json)
}

// confirmDelete shows what will go and asks; an error means don't delete.
func confirmDelete(conn api.JiraConnection, req ops.DeleteRequest) error {
	if !isInteractiveFn() {
		return errConfirmNeeded
	}
	target, err := ops.InspectDelete(conn, req.Key)
	if err != nil {
		return err
	}
	fmt.Println(deleteDescription(target))
	if !promptConfirm("Permanently delete? This cannot be undone.") {
		return errors.New("cancelled")
	}
	return nil
}

// deleteDescription is "KEY: summary" plus any subtasks, for confirmation.
func deleteDescription(res ops.DeleteResult) string {
	text := fmt.Sprintf("%s: %s", tui.KeyStyle.Render(termsafe.Line(res.Key)), termsafe.Line(res.Summary))
	if len(res.Subtasks) > 0 {
		text += "\nSubtasks: " + strings.Join(termsafe.Lines(res.Subtasks), ", ")
	}
	return text
}

// reportDelete prints what was (or would be) deleted.
func reportDelete(res ops.DeleteResult, jsonOut bool) {
	if jsonOut {
		printJSONLine(deleteJSON{Key: res.Key, Summary: res.Summary, Subtasks: res.Subtasks, Status: res.Status})
		return
	}
	verb := "Deleted"
	if res.Status == ops.StatusDryRun {
		verb = "Dry run: would delete"
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render(verb), deleteDescription(res))
}

// menuDeleteTicket is the menu's "Delete Ticket" action: show the ticket,
// confirm, and ask about subtasks only when it has some.
func menuDeleteTicket(conn api.JiraConnection) {
	key := pickTicketKey(conn, "Ticket key to delete: ")
	if key == "" {
		fmt.Println("Cancelled.")
		return
	}
	target, err := ops.InspectDelete(conn, key)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	fmt.Println(deleteDescription(target))
	req := ops.DeleteRequest{Key: key, Cascade: len(target.Subtasks) > 0 && promptConfirm("Delete its subtasks too?")}
	if !promptConfirm("Permanently delete? This cannot be undone.") {
		fmt.Println("Cancelled.")
		return
	}
	res, err := ops.DeleteTicket(conn, req, false)
	if err != nil {
		printMenuErr("%s", termsafe.Clean(err.Error()))
		return
	}
	reportDelete(res, false)
}
