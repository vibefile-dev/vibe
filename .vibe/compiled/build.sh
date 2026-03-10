#!/bin/bash
set -euo pipefail

# Preflight checks
if ! command -v go &> /dev/null; then
  echo "error: go is required but not installed"
  exit 2
fi

# Task: compile the Go binary named vibe from the module root
go build -o vibe .