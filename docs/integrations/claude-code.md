# Claude Code

Recommended:

```bash
whichrepo setup claude --scope user --workspace /path/to/workspace
```

Manual alternative:

Register WhichRepo for the current user:

```bash
claude mcp add whichrepo --scope user -- whichrepo mcp --workspace /path/to/workspace
claude mcp get whichrepo
claude mcp list
```

For a project-scoped configuration, add this to `.mcp.json`:

```json
{
  "mcpServers": {
    "whichrepo": {
      "type": "stdio",
      "command": "whichrepo",
      "args": ["mcp", "--workspace", "/path/to/workspace"]
    }
  }
}
```
