# CLI Agent Guide

This repo contains the Go CLI for `hackctl`.

## Scope

The current shipped commands are:

- `hackctl create`
- `hackctl start`
- `hackctl share`
- `hackctl deploy`
- `hackctl status`
- `hackctl destroy`

## Code Areas

- `cmd/` - command definitions and entry flow
- `internal/config/` - `hackctl.config.json`, `.env`, and runtime state helpers
- `internal/templates/` - template registry
- `internal/output/` - terminal styling and status UI

## Working Rules

- Keep CLI changes small and behavior-focused.
- Preserve cross-platform behavior for Windows, macOS, and Linux.
- Prefer actionable beginner-facing error messages.
- Avoid interactive shell workflows.
- If you change template expectations, verify the matching template contract in `../templates/`.

## Verification

Run the smallest relevant checks:

- `go test ./...`
- `go build ./cmd/hackctl`

If a change affects release packaging or platform-specific behavior, inspect `.github/workflows/` before changing asset names or output paths.
