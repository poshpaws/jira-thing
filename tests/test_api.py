"""Tests for jira_client.api module."""

from __future__ import annotations

import pytest
import responses

from jira_client.api import ISSUE_ENDPOINT, JiraConnection, create_issue, fetch_issue

FAKE_URL = "https://test.atlassian.net"


@pytest.fixture()
def conn() -> JiraConnection:
    return JiraConnection(base_url=FAKE_URL, email="a@b.com", api_token="tok")


class TestFetchIssue:
    @responses.activate
    def test_fetch_issue_returns_json(self, conn: JiraConnection) -> None:
        expected = {"key": "PROJ-1", "fields": {"summary": "Test"}}
        responses.add(
            responses.GET,
            f"{FAKE_URL}{ISSUE_ENDPOINT}/PROJ-1",
            json=expected,
            status=200,
        )
        result = fetch_issue(conn, "PROJ-1")
        assert result == expected

    @responses.activate
    def test_fetch_issue_raises_on_404(self, conn: JiraConnection) -> None:
        responses.add(
            responses.GET,
            f"{FAKE_URL}{ISSUE_ENDPOINT}/BAD-1",
            json={"errorMessages": ["not found"]},
            status=404,
        )
        with pytest.raises(Exception, match="404"):
            fetch_issue(conn, "BAD-1")


class TestCreateIssue:
    @responses.activate
    def test_create_issue_returns_key(self, conn: JiraConnection) -> None:
        expected = {"key": "PROJ-2", "self": "https://test.atlassian.net/rest/api/3/issue/2"}
        responses.add(
            responses.POST,
            f"{FAKE_URL}{ISSUE_ENDPOINT}",
            json=expected,
            status=201,
        )
        result = create_issue(conn, {"summary": "New ticket"})
        assert result["key"] == "PROJ-2"

    @responses.activate
    def test_create_issue_raises_on_400(self, conn: JiraConnection) -> None:
        responses.add(
            responses.POST,
            f"{FAKE_URL}{ISSUE_ENDPOINT}",
            json={"errors": {"summary": "required"}},
            status=400,
        )
        with pytest.raises(Exception, match="400"):
            create_issue(conn, {})
