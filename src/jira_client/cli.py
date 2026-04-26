"""CLI entry point for the Jira client POC tool."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from jira_client.api import JiraConnection, create_issue, fetch_issue
from jira_client.auth import clear_credentials, get_credentials
from jira_client.template import build_template, load_template, save_template


def _build_connection() -> JiraConnection:
    """Build a JiraConnection from stored credentials."""
    url, email, token = get_credentials()
    return JiraConnection(base_url=url, email=email, api_token=token)


def _handle_template(args: argparse.Namespace) -> None:
    """Fetch a ticket and save it as a template."""
    conn = _build_connection()
    print(f"Fetching {args.ticket}...")
    issue = fetch_issue(conn, args.ticket)
    template = build_template(issue)
    output = save_template(template, Path(args.output) if args.output else None)
    print(f"Template saved to {output}")
    print(json.dumps(template, indent=2))


def _handle_create(args: argparse.Namespace) -> None:
    """Create a new ticket from the template."""
    template = load_template(Path(args.template) if args.template else None)
    summary = input("Enter ticket summary: ").strip()
    description = input("Enter ticket description: ").strip()

    if not summary:
        print("Summary is required.", file=sys.stderr)
        sys.exit(1)

    template["summary"] = summary
    template["description"] = {
        "type": "doc",
        "version": 1,
        "content": [
            {
                "type": "paragraph",
                "content": [{"type": "text", "text": description}],
            }
        ],
    }

    conn = _build_connection()
    result = create_issue(conn, template)
    print(f"Created ticket: {result['key']}")
    print(f"URL: {conn.base_url}/browse/{result['key']}")


def _build_parser() -> argparse.ArgumentParser:
    """Build the CLI argument parser."""
    parser = argparse.ArgumentParser(
        prog="jira-client",
        description="POC tool to template and create Jira tickets.",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    tpl = sub.add_parser("template", help="Fetch a ticket and save as a template")
    tpl.add_argument("ticket", help="Jira ticket key (e.g. PROJ-123)")
    tpl.add_argument("-o", "--output", help="Output file path", default=None)

    create = sub.add_parser("create", help="Create a new ticket from a template")
    create.add_argument("-t", "--template", help="Path to template file", default=None)

    sub.add_parser("clear-auth", help="Clear stored credentials")

    return parser


def main() -> None:
    """CLI entry point."""
    parser = _build_parser()
    args = parser.parse_args()

    handlers = {
        "template": _handle_template,
        "create": _handle_create,
        "clear-auth": lambda _: clear_credentials(),
    }
    handlers[args.command](args)


if __name__ == "__main__":
    main()
