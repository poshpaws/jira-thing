package main

import (
	"flag"
	"fmt"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/auth"
	"jira-thing/internal/tui"
)

// runDiagnose tests Jira API connectivity using stored credentials.
func runDiagnose(args []string) {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	userID := fs.Bool("userid", false, "Print only the current user's accountId")
	_ = fs.Parse(args)

	if *userID {
		conn := mustConnect()
		me, err := api.FetchMyself(conn)
		if err != nil {
			fatal("fetching current user: %v", err)
		}
		fmt.Println(me["accountId"])
		return
	}

	fmt.Println("Running diagnostics...")
	fmt.Println()

	conn, err := buildConnection()
	if err != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗ Credentials:"), err.Error())
		return
	}
	fmt.Printf("  %s\n", tui.SuccessStyle.Render("✓ Credentials loaded from keyring"))
	fmt.Printf("    URL:   %s\n", conn.BaseURL)
	fmt.Printf("    Email: %s\n", conn.Email)
	fmt.Printf("    Token: %s\n", maskToken(conn.APIToken))

	if err := auth.ValidateToken(conn.APIToken); err != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗ Token format:"), err.Error())
		return
	}
	fmt.Printf("  %s\n", tui.SuccessStyle.Render("✓ Token format valid"))
	fmt.Println()

	me, err := api.FetchMyself(conn)
	if err != nil {
		fmt.Printf("  %s %s\n", tui.ErrorStyle.Render("✗ API connection:"), formatDiagError(err))
		return
	}
	displayName, _ := me["displayName"].(string)
	accountID, _ := me["accountId"].(string)
	fmt.Printf("  %s\n", tui.SuccessStyle.Render("✓ API connection successful"))
	fmt.Printf("    User:      %s\n", displayName)
	fmt.Printf("    AccountId: %s\n", accountID)
	fmt.Println()
	fmt.Println(tui.SuccessStyle.Render("All checks passed."))
}

// formatDiagError adds helpful context for common API errors.
func formatDiagError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "401") {
		return msg + "\n    → API token may be invalid or expired. Regenerate at https://id.atlassian.com/manage-profile/security/api-tokens"
	}
	if strings.Contains(msg, "403") {
		return msg + "\n    → Check that the email matches the token owner's Atlassian account"
	}
	return msg
}

// maskToken shows the first and last 3 characters of a token with ellipsis between.
func maskToken(token string) string {
	if len(token) <= 6 {
		return "***"
	}
	return token[:3] + "..." + token[len(token)-3:]
}
