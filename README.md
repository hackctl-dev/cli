# hackctl CLI

`hackctl` is the Go CLI for the core hackctl workflow:

```bash
hackctl create
hackctl create --template mern my-app
hackctl start
hackctl share
hackctl deploy --target root@203.0.113.10 --key ~/.ssh/id_ed25519
hackctl status
hackctl destroy
```

## Install

The hosted installers download the latest GitHub release and verify checksums before installing.

Supported release platforms:

- Windows x64
- macOS Intel
- macOS Apple Silicon
- Linux x64
- Linux ARM64

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

## Current Commands

### `hackctl create`

Scaffolds a project from an official template.

```bash
hackctl create
hackctl create --template mern my-app
hackctl create -t mern .
```

Current behavior:

- running `hackctl create` with no args opens an interactive prompt for template and path
- supports official templates: `mern`, `pern`, `next-supabase`, `sveltekit-supabase`, `nuxt-supabase`
- requires `git`, `node >= 20`, and `npm >= 10`
- clones from `https://github.com/hackctl-dev/templates.git`
- copies the selected template subdirectory into the target path
- validates `hackctl.config.json` service and share settings before continuing
- enforces npm-based run commands for official templates
- scaffolds the template's `deploy` block, currently `runtime: PM2` and `mode: dev`
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
- starts all configured services in parallel
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

### `hackctl deploy`

Deploys the current project to a remote Linux VPS with PM2 and a remote Cloudflare tunnel.

```bash
hackctl deploy --target root@203.0.113.10 --key ~/.ssh/id_ed25519
hackctl deploy
```

Current behavior:

- must be run inside a hackctl project
- requires `deploy.runtime` in `hackctl.config.json`
- currently supports `deploy.runtime: PM2` only
- currently supports `deploy.mode: dev` only
- requires local `ssh` and `scp`
- currently targets Ubuntu or Debian VPS hosts with `root` or `sudo`
- uploads project files while excluding `.git`, `.hackctl`, `node_modules`, and local build output
- installs Node.js 20+, npm 10+, PM2, and `cloudflared` on the remote host when needed
- starts the configured services remotely with PM2 using their existing `npm run dev` commands
- exposes the configured public service through a remote Cloudflare tunnel
- writes deploy metadata to `.hackctl/deploy.json`
- reuses saved `target` and `keyPath` from `.hackctl/deploy.json` when flags are omitted on later deploys

### `hackctl status`

Shows the saved deployed-project details.

```bash
hackctl status
```

Current behavior:

- must be run inside a hackctl project
- reads `.hackctl/deploy.json`
- shows the saved target, runtime, mode, public URL, and deployed services
- does not perform a live remote health check yet

### `hackctl destroy`

Destroys the remote deployment created by `hackctl deploy`.

```bash
hackctl destroy
```

Current behavior:

- must be run inside a hackctl project
- requires an existing `.hackctl/deploy.json`
- fails with `no services are deployed` if saved deploy state is missing
- stops remote PM2 apps, stops the deploy-owned tunnel, removes the remote project directory, and clears `.hackctl/deploy.json`

## Development

Requirements:

- Go 1.24.x
- Git
- OpenSSH client (`ssh`, `scp`)
- Node.js 20+ and npm 10+

Build:

```bash
go build ./cmd/hackctl
```

## Local VM Testing

You can test `hackctl deploy`, `hackctl status`, and `hackctl destroy` locally with Vagrant and VirtualBox using an Ubuntu Server 24.04 VM.

Requirements:

- Vagrant
- VirtualBox
- local OpenSSH client (`ssh`, `scp`)
- a project-local SSH keypair at the workspace root:
  - `../key`
  - `../key.pub`

Example disposable VM setup:

1. Create a temporary directory for the VM and add a `Vagrantfile` like this:

```ruby
Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-24.04"
  config.vm.hostname = "hackctl-test-vm"
  config.vm.network "private_network", ip: "192.168.56.10"
  config.vm.synced_folder ".", "/vagrant", disabled: true
  config.vm.boot_timeout = 600

  config.vm.provider "virtualbox" do |vb|
    vb.name = "hackctl-test-vm"
    vb.memory = 2048
    vb.cpus = 2
  end

  config.vm.provision "shell", inline: <<-'SHELL'
    set -eu
    install -d -m 700 /home/vagrant/.ssh
    cat >> /home/vagrant/.ssh/authorized_keys <<'EOF'
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOGygHIrCVh3znwYpFbWqoOBI2e367/SvHsHrn1MW4uE hackctl-multipass-test
    EOF
    chmod 600 /home/vagrant/.ssh/authorized_keys
    chown -R vagrant:vagrant /home/vagrant/.ssh
  SHELL
end
```

2. Boot the VM:

```bash
vagrant up
```

3. Verify direct SSH with the project-local key:

```bash
ssh -i ../key -o StrictHostKeyChecking=no vagrant@192.168.56.10 "hostname && id -un"
```

4. From a test project directory, run:

```bash
hackctl deploy --target vagrant@192.168.56.10 --key ../key
hackctl status
hackctl destroy
```

5. Tear the VM down when done:

```bash
vagrant destroy -f
```

Notes:

- the current deploy path expects an Ubuntu or Debian-style guest with `apt-get`
- the current deploy path expects a user with `sudo` or `root`
- `status` reads local saved deploy metadata from `.hackctl/deploy.json`
- `destroy` requires a prior successful deploy and returns `no services are deployed` when local deploy state is missing

Test:

```bash
go test ./...
```

## Repository Layout

- `cmd/` - Cobra commands
- `internal/config/` - project config, env loading, runtime state
- `internal/config/` - project config, env loading, local runtime state, deploy state
- `internal/templates/` - official template registry
- `internal/output/` - terminal UI and styling
