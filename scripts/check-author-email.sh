#!/usr/bin/env bash
set -euo pipefail

email=$(git config user.email)

if [[ "$email" != *@swissunixsupport.com ]]; then
    echo "ERROR: Commits are only allowed from @swissunixsupport.com email addresses."
    exit 1
fi
