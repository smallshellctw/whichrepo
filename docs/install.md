# Install WhichRepo

WhichRepo is distributed as a self-contained native executable. End users do not need Go, Node.js, Python, Java, SQLite, or Docker.

## macOS and Linux

```bash
curl -fsSL https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.sh | sh
```

The default destination is `~/.local/bin`. Override it when needed:

```bash
INSTALL_DIR=/usr/local/bin sh scripts/install.sh
```

Install a fixed version:

```bash
WHICHREPO_VERSION=v0.1.0 sh scripts/install.sh
```

## Windows

Run PowerShell:

```powershell
irm https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.ps1 | iex
```

The default destination is `%USERPROFILE%\.local\bin`. Add that directory to `PATH` if it is not already available.

## GitHub Releases

Every release contains macOS, Linux, and Windows archives for amd64 and arm64 plus `checksums.txt`. Both installers verify the checksum before copying the executable.

After extracting an archive:

```bash
whichrepo --help
whichrepo doctor
```

## Configure coding agents

After initializing a workspace, configure every supported client detected on the machine:

```bash
whichrepo init /path/to/workspace
whichrepo setup auto --workspace /path/to/workspace
```

Use `whichrepo setup codex`, `whichrepo setup claude`, or `whichrepo setup cursor` to target one client. Add `--dry-run` to preview. See [`AI_INSTALL.md`](../AI_INSTALL.md) for a prompt designed to be handed directly to an AI coding agent.

## Docker

```bash
docker run --rm -i \
  -v "$PWD:/workspace:ro" \
  ghcr.io/smallshellctw/whichrepo:latest \
  route --workspace /workspace "describe your task"
```

Use a writable volume for the database when repeatedly indexing the same workspace.

## Build from source

This path is for contributors, not end users:

```bash
git clone https://github.com/smallshellctw/whichrepo.git
cd whichrepo
make build
```

`make build` compiles only the supported grammar subset and forces `CGO_ENABLED=0`.

## Uninstall

Remove the installed executable and, optionally, its cache:

```bash
rm ~/.local/bin/whichrepo
rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/whichrepo"
```

On macOS the cache is normally under `~/Library/Caches/whichrepo`. On Windows it uses the OS user cache directory.
