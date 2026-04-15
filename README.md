# hackctl CLI

`hackctl` is the Go CLI for the core hackctl workflow:

```bash
hackctl create --template mern my-app
hackctl start
hackctl share
```

## Quickstart

```bash
hackctl create --template mern my-app
cd my-app
hackctl start
hackctl share
```

Notes:

- Run `hackctl start` and `hackctl share` from the project root (where `hackctl.config.json` exists).
- Press `Ctrl+C` to stop `start` services or stop an active `share` tunnel.
- If a required dependency is missing, hackctl prints a short install link.

## Current Commands

### `hackctl create`

Scaffolds a project from an official template.

```bash
hackctl create --template mern my-app
hackctl create -t mern .
```

Current behavior:

- supports official templates: `mern`, `pern`, `next-supabase`, `sveltekit-supabase`, `nuxt-supabase`
- requires `git`, `node >= 20`, and `npm >= 10`
- clones from `https://github.com/hackctl-dev/templates.git`
- copies the selected template subdirectory into the target path
- validates `hackctl.config.json` service and share settings before continuing
- enforces npm-based run commands for official templates
- expects templates to ignore `.hackctl/` in `.gitignore`
- installs Node dependencies for detected services
- fails if the target directory already exists

### `hackctl start`

Starts the current hackctl project from `hackctl.config.json`.

```bash
hackctl start
```

Current behavior:

- must be run inside a hackctl project
- requires `node >= 20` and `npm >= 10`
- installs missing dependencies when needed
- starts all configured services
- waits for each service port to become reachable before marking it running
- writes runtime state to `.hackctl/state.json`
- shows a compact terminal UI instead of raw logs
- stops child processes on `Ctrl+C`
- missing or unsupported dependency errors include an official install link
- shows concise error details when install/start/readiness fails

### `hackctl share`

Shares the default frontend service publicly with Cloudflare Quick Tunnel.

```bash
hackctl share
```

Current behavior:

- reads the share target from `hackctl.config.json`
- checks that the target port is reachable
- uses installed `cloudflared` or downloads a cached copy
- prints the public `trycloudflare.com` URL
- updates `.hackctl/state.json`
- clears tunnel state on exit
- treats unexpected tunnel exits as errors and shows concise failure details

## Development

Requirements:

- Go 1.24.x
- Git
- Node.js 20+ and npm 10+

Build:

```bash
go build ./cmd/hackctl
```

Test:

```bash
go test ./...
```

## Repository Layout

- `cmd/` - Cobra commands
- `internal/config/` - project config, env loading, runtime state
- `internal/templates/` - official template registry
- `internal/output/` - terminal UI and styling

## Status

This repo implements the current MVP flow. Planned roadmap items like deploy, status, and destroy are not shipped yet.
