#!/usr/bin/env bash
set -euo pipefail

commit_msg_file="$1"

if grep -qi "co-authored-by.*claude" "$commit_msg_file"; then
    echo "ERROR: Commit message contains 'Co-authored-by Claude'. This is not allowed."
    exit 1
fi
