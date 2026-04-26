"""Template generation and loading for Jira tickets."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

TEMPLATE_FILE = "ticket_template.json"

TEMPLATE_FIELDS = (
    "project",
    "issuetype",
    "priority",
    "labels",
    "components",
    "assignee",
)


def build_template(issue_data: dict[str, Any]) -> dict[str, Any]:
    """Extract reusable fields from a Jira issue into a template.

    Args:
        issue_data: Raw Jira issue JSON.

    Returns:
        A dict containing only the template-worthy fields.
    """
    fields = issue_data.get("fields", {})
    template: dict[str, Any] = {}
    for key in TEMPLATE_FIELDS:
        if key in fields and fields[key] is not None:
            template[key] = fields[key]
    return template


def save_template(template: dict[str, Any], path: Path | None = None) -> Path:
    """Save template to a JSON file.

    Args:
        template: The template dict to save.
        path: Optional output path. Defaults to TEMPLATE_FILE in cwd.

    Returns:
        The path the template was written to.
    """
    output = path or Path(TEMPLATE_FILE)
    output.write_text(json.dumps(template, indent=2) + "\n")
    return output


def load_template(path: Path | None = None) -> dict[str, Any]:
    """Load a template from a JSON file.

    Args:
        path: Optional path to template. Defaults to TEMPLATE_FILE in cwd.

    Returns:
        The template dict.

    Raises:
        FileNotFoundError: If the template file does not exist.
    """
    source = path or Path(TEMPLATE_FILE)
    if not source.exists():
        raise FileNotFoundError(f"Template not found: {source}")
    return json.loads(source.read_text())
