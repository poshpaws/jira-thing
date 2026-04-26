"""Jira REST API client."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import requests

ISSUE_ENDPOINT = "/rest/api/3/issue"
REQUEST_TIMEOUT_SECONDS = 30


@dataclass(frozen=True)
class JiraConnection:
    """Holds Jira connection details."""

    base_url: str
    email: str
    api_token: str


def fetch_issue(conn: JiraConnection, issue_key: str) -> dict[str, Any]:
    """Fetch a Jira issue by key.

    Args:
        conn: Jira connection details.
        issue_key: The ticket key (e.g. PROJ-123).

    Returns:
        The issue JSON response.

    Raises:
        requests.HTTPError: If the API request fails.
    """
    url = f"{conn.base_url}{ISSUE_ENDPOINT}/{issue_key}"
    resp = requests.get(
        url,
        auth=(conn.email, conn.api_token),
        headers={"Accept": "application/json"},
        timeout=REQUEST_TIMEOUT_SECONDS,
    )
    resp.raise_for_status()
    return resp.json()


def create_issue(conn: JiraConnection, fields: dict[str, Any]) -> dict[str, Any]:
    """Create a new Jira issue.

    Args:
        conn: Jira connection details.
        fields: The issue fields payload.

    Returns:
        The created issue JSON response.

    Raises:
        requests.HTTPError: If the API request fails.
    """
    url = f"{conn.base_url}{ISSUE_ENDPOINT}"
    resp = requests.post(
        url,
        auth=(conn.email, conn.api_token),
        headers={"Accept": "application/json", "Content-Type": "application/json"},
        json={"fields": fields},
        timeout=REQUEST_TIMEOUT_SECONDS,
    )
    resp.raise_for_status()
    return resp.json()
