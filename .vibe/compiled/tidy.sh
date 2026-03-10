#!/bin/bash
set -euo pipefail

# Preflight checks
if ! command -v go &> /dev/null; then
  echo "error: go is required but not installed"
  exit 2
fi

# Task: run go mod tidy
go mod tidy