#!/bin/bash
set -euo pipefail

# Preflight checks
if ! command -v cp > /dev/null 2>&1; then
  echo "error: cp is required but not installed"
  exit 2
fi

# Task: copy the binary to /usr/local/bin
BINARY="vibe"

if [ ! -f "$BINARY" ]; then
  echo "error: binary '$BINARY' not found in current directory"
  exit 1
fi

cp "$BINARY" /usr/local/bin/vibe
echo "Copied '$BINARY' to /usr/local/bin/vibe"