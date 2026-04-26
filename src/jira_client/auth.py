"""Secure credential management for Jira using keyring."""

from __future__ import annotations

import contextlib
import getpass

import keyring
from keyring.errors import PasswordDeleteError

KEYRING_SERVICE = "jira-client-poc"
KEY_URL = "jira_url"
KEY_EMAIL = "jira_email"
KEY_TOKEN = "jira_api_token"


def get_credentials() -> tuple[str, str, str]:
    """Retrieve stored Jira credentials, prompting if missing.

    Returns:
        Tuple of (base_url, email, api_token).
    """
    url = keyring.get_password(KEYRING_SERVICE, KEY_URL)
    email = keyring.get_password(KEYRING_SERVICE, KEY_EMAIL)
    token = keyring.get_password(KEYRING_SERVICE, KEY_TOKEN)

    if not all([url, email, token]):
        url, email, token = _prompt_and_store()

    return url, email, token  # type: ignore[return-value]


def _prompt_and_store() -> tuple[str, str, str]:
    """Prompt user for credentials and store them in keyring."""
    print("Jira credentials not found. Please enter them now.")
    url = input("Jira base URL (e.g. https://yourorg.atlassian.net): ").strip().rstrip("/")
    email = input("Jira email: ").strip()
    token = getpass.getpass("Jira API token: ").strip()

    if not all([url, email, token]):
        raise ValueError("All credential fields are required.")

    keyring.set_password(KEYRING_SERVICE, KEY_URL, url)
    keyring.set_password(KEYRING_SERVICE, KEY_EMAIL, email)
    keyring.set_password(KEYRING_SERVICE, KEY_TOKEN, token)
    print("Credentials stored securely in keyring.")
    return url, email, token


def clear_credentials() -> None:
    """Remove all stored Jira credentials from keyring."""
    for key in (KEY_URL, KEY_EMAIL, KEY_TOKEN):
        with contextlib.suppress(PasswordDeleteError):
            keyring.delete_password(KEYRING_SERVICE, key)
    print("Credentials cleared.")
