# Install WhichRepo with an AI coding agent

This file is written as an executable checklist for Codex, Claude Code, Cursor, OpenCode, Gemini CLI, and other local coding agents.

## Copy this prompt

Replace `/absolute/path/to/workspace`, then paste the prompt into your coding agent:

```text
Install WhichRepo for the workspace /absolute/path/to/workspace using only the official repository:
https://github.com/smallshellctw/whichrepo

Follow AI_INSTALL.md from that repository. Requirements:
1. Detect macOS, Linux, or Windows and use the official checksum-verifying installer.
2. Do not install Go, Node.js, Python, Java, SQLite, or Docker.
3. Run `whichrepo --version`.
4. If `.whichrepo.yaml` does not exist, run `whichrepo init /absolute/path/to/workspace`.
5. Run `whichrepo setup auto --workspace /absolute/path/to/workspace` to configure supported local AI clients. Preserve existing MCP servers and backups.
6. Run `whichrepo doctor --workspace /absolute/path/to/workspace` and resolve any warning.
7. Restart is a user action: tell me which clients must be restarted.
8. Verify `workspace_status`, `list_projects`, and one `route_task` call. Do not modify project source code.
9. Never request or store an AI provider key; local routing must work without one.
```

## Commands for the agent

macOS/Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.ps1 | iex
```

Then:

```bash
whichrepo init /absolute/path/to/workspace
whichrepo setup auto --workspace /absolute/path/to/workspace
whichrepo doctor --workspace /absolute/path/to/workspace
```

Use `whichrepo setup auto --dry-run --workspace /absolute/path/to/workspace` when the user wants to preview configuration changes. Cursor configuration is backed up before it is changed. Codex and Claude Code are configured through their official CLI commands.

## Verification contract

Installation is complete only when:

- `whichrepo doctor` reports `cgo_required: false` and no runtime dependencies;
- at least one project is indexed;
- the configured client exposes `route_task`, `refresh_index`, `list_projects`, and `workspace_status`;
- a local-only route succeeds without an API key;
- no source file was modified.
