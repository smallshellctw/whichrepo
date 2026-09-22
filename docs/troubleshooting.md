# Troubleshooting

Start with:

```bash
whichrepo doctor --workspace /path/to/workspace
whichrepo config validate --workspace /path/to/workspace
whichrepo config paths --workspace /path/to/workspace
```

## `whichrepo` is not found after installation

Add the installer destination to `PATH`. The default is `~/.local/bin` on macOS/Linux and `%USERPROFILE%\.local\bin` on Windows.

## No projects are indexed

- Confirm the workspace is the common parent of the repositories.
- Run `whichrepo index --workspace /path/to/workspace`.
- Check `workspace.include` and `workspace.exclude`.
- Add explicit project paths when repositories are nested more deeply than normal discovery.

## The wrong repository ranks first

- Add a clear description and business aliases in `.whichrepo.yaml`.
- Check package identifiers and dependency links in `whichrepo dashboard`.
- Add a representative task to a private evaluation dataset before changing global scoring behavior.

## Cursor cannot see MCP tools

Run:

```bash
whichrepo setup cursor --workspace /path/to/workspace
```

Then run **Developer: Reload Window** or restart Cursor. Verify that `route_task`, `refresh_index`, `list_projects`, and `workspace_status` appear. Cursor configuration is backed up before changes.

## Codex or Claude Code cannot start the server

- Use `whichrepo setup codex` or `whichrepo setup claude` again after moving the binary.
- Confirm the configured command is an absolute executable path.
- Run that command with `mcp --workspace ...` from a terminal and check stderr.

## Jev returns auth or billing errors

Local routing remains usable. Confirm that the key belongs to the selected provider and that the provider account can make requests. Use `routing.provider: local` to disable remote decisions.

## Dashboard refuses the listen address

This is intentional. The Dashboard accepts only `127.0.0.1`, `localhost`, or `::1` to avoid exposing local repository metadata to the network.

