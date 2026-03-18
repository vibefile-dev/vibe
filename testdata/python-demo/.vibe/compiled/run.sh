#!/bin/bash
set -euo pipefail

# Preflight
if ! command -v uv &> /dev/null; then
  echo "error: uv is required but not installed"
  exit 2
fi

# Run the demo CLI entry point
uv run demo