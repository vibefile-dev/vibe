"""Shared fixtures for the test suite."""

from __future__ import annotations

import pytest


@pytest.fixture()
def long_text() -> str:
    """A deterministic long string useful for truncation tests."""
    return "the quick brown fox jumps over the lazy dog " * 5
