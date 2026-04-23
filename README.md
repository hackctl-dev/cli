# hackctl CLI

`hackctl` is the Go CLI for the core hackctl workflow.

## Install

Windows x64:

```powershell
irm https://hackctl.dev/install.ps1 | iex
```

macOS Intel and Apple Silicon:

```bash
curl -fsSL https://hackctl.dev/install.sh | sh
```

Linux x64 and ARM64:

```bash
curl -fsSL https://hackctl.dev/install.sh | sh
```

Verify:

```bash
hackctl --version
```

## Quickstart

```bash
hackctl create
hackctl create --template mern my-app
cd my-app
hackctl start
hackctl share
hackctl deploy --target root@203.0.113.10 --key ~/.ssh/id_ed25519
hackctl status
hackctl destroy
```

Notes:

- Run `hackctl start` and `hackctl share` from the project root (where `hackctl.config.json` exists).
- Run `hackctl deploy`, `hackctl status`, and `hackctl destroy` from the project root too.
- Press `Ctrl+C` to stop `start` services or stop an active `share` tunnel.
- If a required dependency is missing, hackctl prints a short install link.

## Commands

### `hackctl create`

Scaffolds a project from an official template.

```bash
hackctl create
hackctl create --template mern my-app
```

### `hackctl start`

Starts the current hackctl project from `hackctl.config.json`.

```bash
hackctl start
```

### `hackctl share`

Shares the default frontend service publicly with Cloudflare Quick Tunnel.

```bash
hackctl share
```

### `hackctl deploy`

Deploys the current project to a remote Linux VPS.

```bash
hackctl deploy --target root@203.0.113.10 --key ~/.ssh/id_ed25519
hackctl deploy
```

### `hackctl status`

Shows the saved deployed-project details.

```bash
hackctl status
```

### `hackctl destroy`

Destroys the remote deployment created by `hackctl deploy`.

```bash
hackctl destroy
```

## Contributing

Found a bug or have a feature request? See [CONTRIBUTING.md](./CONTRIBUTING.md).
