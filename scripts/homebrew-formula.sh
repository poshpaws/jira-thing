#!/usr/bin/env bash
# Print the Homebrew formula for a jira-thing release.
# Usage: scripts/homebrew-formula.sh <version-without-v> <checksums.txt>
# The checksums file is the signed one published with each release.
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <version> <checksums.txt>" >&2
  exit 2
fi
version="$1"
checksums="$2"
base="https://github.com/poshpaws/jira-thing/releases/download/v${version}"

sha() {
  local sum
  sum=$(awk -v f="jira-thing-$1" '$2 == f { print $1 }' "$checksums")
  if [[ ! "$sum" =~ ^[0-9a-f]{64}$ ]]; then
    echo "error: no sha256 for jira-thing-$1 in $checksums" >&2
    exit 1
  fi
  echo "$sum"
}

# Assigned here, not inline in the heredoc, so a missing checksum stops the
# script (set -e) instead of printing a formula with an empty sha256.
darwin_arm64=$(sha darwin-arm64)
darwin_amd64=$(sha darwin-amd64)
linux_arm64=$(sha linux-arm64)
linux_amd64=$(sha linux-amd64)

cat <<FORMULA
class JiraThing < Formula
  desc "Jira and Confluence CLI, TUI and MCP server for agents and humans"
  homepage "https://github.com/poshpaws/jira-thing"
  version "${version}"

  on_macos do
    on_arm do
      url "${base}/jira-thing-darwin-arm64"
      sha256 "${darwin_arm64}"
    end
    on_intel do
      url "${base}/jira-thing-darwin-amd64"
      sha256 "${darwin_amd64}"
    end
  end

  on_linux do
    on_arm do
      url "${base}/jira-thing-linux-arm64"
      sha256 "${linux_arm64}"
    end
    on_intel do
      url "${base}/jira-thing-linux-amd64"
      sha256 "${linux_amd64}"
    end
  end

  def install
    bin.install Dir["jira-thing-*"].first => "jira-thing"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/jira-thing version")
  end
end
FORMULA
