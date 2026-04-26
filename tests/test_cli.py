"""Tests for jira_client.cli module."""

from __future__ import annotations

import json
from typing import Any
from unittest.mock import MagicMock, patch

import pytest

from jira_client.api import JiraConnection
from jira_client.cli import _build_parser, _handle_create, _handle_template

FAKE_CONN = JiraConnection(base_url="https://test.atlassian.net", email="a@b.com", api_token="tok")


class TestHandleTemplate:
    @patch("jira_client.cli._build_connection", return_value=FAKE_CONN)
    @patch("jira_client.cli.fetch_issue")
    def test_saves_template_file(
        self,
        mock_fetch: MagicMock,
        _mock_conn: MagicMock,
        sample_issue: dict[str, Any],
        tmp_path: Any,
    ) -> None:
        mock_fetch.return_value = sample_issue
        output = tmp_path / "tpl.json"
        args = _build_parser().parse_args(["template", "PROJ-123", "-o", str(output)])
        _handle_template(args)
        assert output.exists()
        data = json.loads(output.read_text())
        assert data["project"] == {"key": "PROJ"}


class TestHandleCreate:
    @patch("jira_client.cli._build_connection", return_value=FAKE_CONN)
    @patch("jira_client.cli.create_issue")
    @patch("builtins.input", side_effect=["My summary", "My description"])
    def test_creates_ticket_from_template(
        self,
        _mock_input: MagicMock,
        mock_create: MagicMock,
        _mock_conn: MagicMock,
        tmp_path: Any,
    ) -> None:
        tpl = {"project": {"key": "PROJ"}, "issuetype": {"name": "Task"}}
        tpl_path = tmp_path / "tpl.json"
        tpl_path.write_text(json.dumps(tpl))
        mock_create.return_value = {"key": "PROJ-999"}

        args = _build_parser().parse_args(["create", "-t", str(tpl_path)])
        _handle_create(args)

        call_fields = mock_create.call_args[0][1]
        assert call_fields["summary"] == "My summary"
        assert call_fields["project"] == {"key": "PROJ"}

    @patch("jira_client.cli._build_connection", return_value=FAKE_CONN)
    @patch("builtins.input", side_effect=["", "desc"])
    def test_exits_on_empty_summary(
        self, _mock_input: MagicMock, _mock_conn: MagicMock, tmp_path: Any
    ) -> None:
        tpl_path = tmp_path / "tpl.json"
        tpl_path.write_text(json.dumps({"project": {"key": "X"}}))
        args = _build_parser().parse_args(["create", "-t", str(tpl_path)])
        with pytest.raises(SystemExit, match="1"):
            _handle_create(args)


class TestBuildParser:
    def test_template_subcommand(self) -> None:
        args = _build_parser().parse_args(["template", "PROJ-1"])
        assert args.command == "template"
        assert args.ticket == "PROJ-1"

    def test_create_subcommand(self) -> None:
        args = _build_parser().parse_args(["create", "-t", "my.json"])
        assert args.command == "create"
        assert args.template == "my.json"

    def test_requires_subcommand(self) -> None:
        with pytest.raises(SystemExit):
            _build_parser().parse_args([])
