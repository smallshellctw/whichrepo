# Codex

Recommended:

```bash
whichrepo setup codex --workspace /path/to/workspace
```

Manual alternative:

Build or install `whichrepo`, then register its stdio MCP server:

```bash
codex mcp add whichrepo -- whichrepo mcp --workspace /path/to/workspace
codex mcp list
```

For a local development binary, replace `whichrepo` with its absolute path. Keep API keys in the environment; do not put them directly in `config.toml`.

The bundled `whichrepo-task-router` skill teaches Codex to route ambiguous tasks before broad repository exploration and to ask for clarification when the evidence is weak.
