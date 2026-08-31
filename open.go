package main

import (
	"flag"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// runOpen opens a ticket (or the Jira base URL, if no key given) in the default browser.
func runOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fatal("usage: jira-thing open [TICKET-KEY]")
	}
	conn := mustConnect()
	target := conn.BaseURL
	if fs.NArg() > 0 {
		target = conn.BaseURL + "/browse/" + fs.Arg(0)
	}
	if err := openBrowser(target); err != nil {
		fatal("opening browser: %v", err)
	}
	fmt.Printf("Opened %s\n", target)
}

// openBrowser launches the OS default browser at url.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) // #nosec G204 -- url is built from the user's own configured Jira base URL
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url) // #nosec G204 -- see above
	default:
		cmd = exec.Command("xdg-open", url) // #nosec G204 -- see above
	}
	return cmd.Start()
}

// copyToClipboard writes text to the OS clipboard. On Linux this requires
// xclip (X11) to be installed; there's no dependency-free universal fallback.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
