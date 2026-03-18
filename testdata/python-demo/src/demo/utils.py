"""Utility helpers used across the project."""

from __future__ import annotations

import re
import unicodedata


def slugify(text: str) -> str:
    """Convert *text* into a URL-friendly slug.

    >>> slugify("Hello, World!")
    'hello-world'
    """
    text = unicodedata.normalize("NFKD", text).encode("ascii", "ignore").decode()
    text = re.sub(r"[^\w\s-]", "", text).strip().lower()
    return re.sub(r"[-\s]+", "-", text)


def truncate(text: str, *, max_length: int = 80, suffix: str = "…") -> str:
    """Return *text* truncated to *max_length* characters.

    If the text is already short enough it is returned unchanged.

    >>> truncate("short")
    'short'
    >>> truncate("a" * 100, max_length=10)
    'aaaaaaaaa…'
    """
    if max_length < 1:
        raise ValueError("max_length must be >= 1")
    if len(text) <= max_length:
        return text
    return text[: max_length - len(suffix)] + suffix


def parse_bool(value: str) -> bool:
    """Parse common boolean string representations.

    >>> parse_bool("yes")
    True
    >>> parse_bool("0")
    False
    """
    if value.lower() in {"1", "true", "yes", "on"}:
        return True
    if value.lower() in {"0", "false", "no", "off"}:
        return False
    raise ValueError(f"Cannot interpret {value!r} as a boolean")
