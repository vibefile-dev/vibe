"""Tests for demo.utils."""

from __future__ import annotations

import pytest

from demo.utils import parse_bool, slugify, truncate


class TestSlugify:
    def test_simple_phrase(self) -> None:
        assert slugify("Hello, World!") == "hello-world"

    def test_extra_whitespace(self) -> None:
        assert slugify("  lots   of   space  ") == "lots-of-space"

    def test_unicode_characters(self) -> None:
        assert slugify("Ärger mit Übeln") == "arger-mit-ubeln"

    def test_already_a_slug(self) -> None:
        assert slugify("already-clean") == "already-clean"

    def test_empty_string(self) -> None:
        assert slugify("") == ""

    def test_special_characters_only(self) -> None:
        assert slugify("@#$%^&*") == ""


class TestTruncate:
    def test_short_text_unchanged(self) -> None:
        assert truncate("hello") == "hello"

    def test_exact_length_unchanged(self) -> None:
        text = "a" * 80
        assert truncate(text, max_length=80) == text

    def test_long_text_truncated(self, long_text: str) -> None:
        result = truncate(long_text, max_length=20)
        assert len(result) == 20
        assert result.endswith("…")

    def test_custom_suffix(self) -> None:
        result = truncate("a" * 50, max_length=10, suffix="...")
        assert result == "a" * 7 + "..."

    def test_max_length_must_be_positive(self) -> None:
        with pytest.raises(ValueError, match="max_length must be >= 1"):
            truncate("anything", max_length=0)


class TestParseBool:
    @pytest.mark.parametrize("value", ["1", "true", "True", "TRUE", "yes", "on"])
    def test_truthy_values(self, value: str) -> None:
        assert parse_bool(value) is True

    @pytest.mark.parametrize("value", ["0", "false", "False", "FALSE", "no", "off"])
    def test_falsy_values(self, value: str) -> None:
        assert parse_bool(value) is False

    def test_invalid_value_raises(self) -> None:
        with pytest.raises(ValueError, match="Cannot interpret"):
            parse_bool("maybe")
