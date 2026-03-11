#!/bin/bash
set -euo pipefail

echo HELLO!!!

# Preflight
if ! command -v go &>/dev/null; then
  echo "error: go is required but not installed"
  exit 2
fi

# Task: run Go tests with race detection and coverage
go test -race -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1