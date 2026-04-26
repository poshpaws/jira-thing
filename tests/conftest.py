"""Shared fixtures for jira_client tests."""

from __future__ import annotations

from typing import Any

import pytest

from jira_client.api import JiraConnection

FAKE_URL = "https://test.atlassian.net"
FAKE_EMAIL = "test@example.com"
FAKE_TOKEN = "fake-token"


@pytest.fixture()
def jira_conn() -> JiraConnection:
    """Provide a test JiraConnection."""
    return JiraConnection(base_url=FAKE_URL, email=FAKE_EMAIL, api_token=FAKE_TOKEN)


@pytest.fixture()
def sample_issue() -> dict[str, Any]:
    """Provide a sample Jira issue API response."""
    return {
        "key": "PROJ-123",
        "fields": {
            "project": {"key": "PROJ"},
            "issuetype": {"name": "Task"},
            "priority": {"name": "Medium"},
            "labels": ["backend"],
            "components": [{"name": "api"}],
            "assignee": {"accountId": "abc123"},
            "summary": "Original summary",
            "description": "Original description",
            "status": {"name": "Open"},
            "created": "2026-01-01T00:00:00.000+0000",
        },
    }
