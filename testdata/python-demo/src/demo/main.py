"""CLI entry-point for the demo project."""

from __future__ import annotations

import sys

from demo.utils import slugify, truncate


def main() -> None:
    if len(sys.argv) < 2:
        print("Usage: demo <text>")
        sys.exit(1)

    text = " ".join(sys.argv[1:])
    slug = slugify(text)
    short = truncate(text, max_length=40)
    print(f"Original : {text}")
    print(f"Slug     : {slug}")
    print(f"Truncated: {short}")


if __name__ == "__main__":
    main()
