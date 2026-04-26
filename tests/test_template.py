"""Tests for jira_client.template module."""

from __future__ import annotations

import json
from typing import Any

import pytest

from jira_client.template import build_template, load_template, save_template


class TestBuildTemplate:
    def test_extracts_template_fields(self, sample_issue: dict[str, Any]) -> None:
        result = build_template(sample_issue)
        assert result["project"] == {"key": "PROJ"}
        assert result["issuetype"] == {"name": "Task"}
        assert result["labels"] == ["backend"]

    def test_excludes_non_template_fields(self, sample_issue: dict[str, Any]) -> None:
        result = build_template(sample_issue)
        assert "summary" not in result
        assert "description" not in result
        assert "status" not in result

    def test_skips_none_fields(self) -> None:
        issue = {"fields": {"project": {"key": "X"}, "priority": None}}
        result = build_template(issue)
        assert "priority" not in result
        assert result["project"] == {"key": "X"}

    def test_handles_empty_fields(self) -> None:
        assert build_template({}) == {}
        assert build_template({"fields": {}}) == {}


class TestSaveAndLoadTemplate:
    def test_round_trip(self, tmp_path: Any) -> None:
        template = {"project": {"key": "PROJ"}, "issuetype": {"name": "Bug"}}
        path = tmp_path / "tpl.json"
        save_template(template, path)
        loaded = load_template(path)
        assert loaded == template

    def test_save_creates_valid_json(self, tmp_path: Any) -> None:
        template = {"labels": ["a", "b"]}
        path = tmp_path / "tpl.json"
        save_template(template, path)
        raw = path.read_text()
        assert json.loads(raw) == template

    def test_load_raises_file_not_found(self, tmp_path: Any) -> None:
        with pytest.raises(FileNotFoundError, match="Template not found"):
            load_template(tmp_path / "missing.json")
