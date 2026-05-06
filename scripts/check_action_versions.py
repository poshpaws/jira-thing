#!/usr/bin/env python3
"""Check GitHub Action versions in a workflow file against latest releases."""

import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


@dataclass
class ActionRef:
    action: str
    current: str
    line: int


def parse_actions(workflow_path: Path) -> list[ActionRef]:
    refs = []
    pattern = re.compile(r'uses:\s+([a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+)@([^\s#]+)')
    for i, line in enumerate(workflow_path.read_text().splitlines(), 1):
        m = pattern.search(line)
        if m:
            refs.append(ActionRef(action=m.group(1), current=m.group(2), line=i))
    return refs


def latest_version(action: str) -> str:
    for endpoint in (
        f"/repos/{action}/releases/latest",
        f"/repos/{action}/tags",
    ):
        result = subprocess.run(
            ["gh", "api", endpoint],
            capture_output=True, text=True
        )
        if result.returncode != 0:
            continue
        data = json.loads(result.stdout)
        if isinstance(data, dict) and data.get("tag_name"):
            return data["tag_name"]
        if isinstance(data, list) and data:
            return data[0]["name"]
    return "unknown"


def main(workflow_path: str) -> None:
    path = Path(workflow_path)
    if not path.exists():
        print(f"error: {workflow_path} not found", file=sys.stderr)
        sys.exit(1)

    refs = parse_actions(path)
    if not refs:
        print("no actions found")
        return

    seen: set[str] = set()
    print(f"{'STATUS':<10} {'ACTION':<40} {'CURRENT':<20} {'LATEST'}")
    print("-" * 95)

    for ref in refs:
        key = f"{ref.action}@{ref.current}"
        if key in seen:
            continue
        seen.add(key)

        latest = latest_version(ref.action)
        status = "OK" if ref.current == latest else "OUTDATED"
        print(f"{status:<10} {ref.action:<40} {ref.current:<20} {latest}")


if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else ".github/workflows/release.yml"
    main(target)
