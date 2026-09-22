# Cursor

Recommended (backs up and merges `mcp.json`):

```bash
whichrepo setup cursor --scope user --workspace /path/to/workspace
```

Manual alternative:

Add WhichRepo as a stdio MCP server in Cursor settings:

```json
{
  "mcpServers": {
    "whichrepo": {
      "command": "whichrepo",
      "args": ["mcp", "--workspace", "/path/to/workspace"]
    }
  }
}
```

Restart Cursor and verify that `route_task`, `refresh_index`, `list_projects`, and `workspace_status` are available.
