#!/bin/bash
set -euo pipefail

# Preflight checks
if ! command -v go &> /dev/null; then
  echo "error: go is required but not installed"
  exit 2
fi

# Run all Go tests in every package with verbose output and race detection
go test -v -race ./...