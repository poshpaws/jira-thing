"""Tests for jira_client.auth module."""

from __future__ import annotations

from unittest.mock import MagicMock, patch

from jira_client.auth import clear_credentials, get_credentials


class TestGetCredentials:
    @patch("jira_client.auth.keyring")
    def test_returns_stored_credentials(self, mock_kr: MagicMock) -> None:
        mock_kr.get_password.side_effect = lambda _svc, key: {
            "jira_url": "https://x.atlassian.net",
            "jira_email": "a@b.com",
            "jira_api_token": "tok",
        }[key]
        url, email, token = get_credentials()
        assert url == "https://x.atlassian.net"
        assert email == "a@b.com"
        assert token == "tok"

    @patch("jira_client.auth._prompt_and_store")
    @patch("jira_client.auth.keyring")
    def test_prompts_when_missing(self, mock_kr: MagicMock, mock_prompt: MagicMock) -> None:
        mock_kr.get_password.return_value = None
        mock_prompt.return_value = ("https://y.atlassian.net", "b@c.com", "tok2")
        url, _email, _token = get_credentials()
        assert url == "https://y.atlassian.net"
        mock_prompt.assert_called_once()


class TestClearCredentials:
    @patch("jira_client.auth.keyring")
    def test_deletes_all_keys(self, mock_kr: MagicMock) -> None:
        clear_credentials()
        assert mock_kr.delete_password.call_count == 3

    @patch("jira_client.auth.keyring")
    def test_ignores_delete_errors(self, mock_kr: MagicMock) -> None:
        import keyring.errors

        mock_kr.delete_password.side_effect = keyring.errors.PasswordDeleteError()
        clear_credentials()  # should not raise
