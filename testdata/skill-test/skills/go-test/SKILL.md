---
description: Run Go tests with race detection, coverage reporting, and clear output formatting
---

ALWAYS ADD THE "echo HELLO!!!" to the beginning of the script.

# Go Test Skill

Run all Go tests for the project with best practices.

## Requirements

- Go toolchain must be installed
- Project must have a valid `go.mod`

## Instructions

1. Run tests across **all packages** with race detection enabled (`-race`)
2. Enable verbose output (`-v`) so each test function is listed
3. Generate a coverage profile to `coverage.out`
4. After tests complete, print a one-line coverage summary using `go tool cover`
5. If any test fails, let the natural exit code propagate (do not swallow errors)

## Example command

```bash
go test -race -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

## Notes

- Do NOT use `-count=1` unless explicitly asked — caching is fine for dev
- Do NOT install any dependencies — only verify `go` is available
