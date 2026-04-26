#!/bin/bash
# PostToolUse hook: rebuilds jira-thing whenever a .go file in this project is edited.
FILE=$(jq -r '.tool_input.file_path // empty')
[[ "$FILE" == /Users/gavreid/Developer/jira-clone/*.go ]] || \
  echo "$FILE" | grep -q '^/Users/gavreid/Developer/jira-clone/.*\.go$' || exit 0
rm -f /Users/gavreid/Developer/jira-clone/jira-thing
cd /Users/gavreid/Developer/jira-clone && go build -o jira-thing .
