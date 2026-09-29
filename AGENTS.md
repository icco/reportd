# AGENTS.md

Guidance for coding agents working on reportd.

## Project Overview

A collector daemon written in Go (`go.icco.me/reportd`) for ingesting browser reporting API payloads (such as Content Security Policy violations and deprecation reports).

## Commands

```sh
go test ./...    # Run tests
go vet ./...     # Vet code
go run main.go   # Run locally (port 8080 by default)
go build .       # Build binary
```

## Architecture & Conventions

- `main.go` — Server entrypoint, Chi router, and report parsing handlers.
- Follow icco Go conventions (`go.icco.me/gutil` for logging and structured JSON).
- PR titles and commits must follow Conventional Commits with lowercase subjects.
- Ensure all tests pass before submitting PRs.
