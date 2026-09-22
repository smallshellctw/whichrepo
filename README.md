<div align="center">

# WhichRepo

**Describe the task. Find the repo. Show the evidence.**

Language-agnostic, local-first task routing for microservices, polyrepos, monorepos, and coding agents.

[![CI](https://github.com/smallshellctw/whichrepo/actions/workflows/ci.yml/badge.svg)](https://github.com/smallshellctw/whichrepo/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/smallshellctw/whichrepo)](https://github.com/smallshellctw/whichrepo/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-22c55e.svg)](LICENSE)

[30-second start](#30-second-start) · [Install](#install) · [Agents](#coding-agent-integration) · [Languages](#language-support) · [Privacy](#privacy) · [中文](README.zh-CN.md)

</div>

![WhichRepo signal-map dashboard](assets/whichrepo-dashboard.png)

Coding agents can edit code once they have the right context. In a workspace with dozens of services, the expensive first question is often: **which repositories should own this task?**

WhichRepo indexes the workspace locally, ranks the likely repositories, follows cross-service dependencies, and shows the evidence behind every result. It works without an account, API key, network connection, Go, Node.js, Python, Java, or a database server.

```text
"Add retry limits to checkout webhooks"
                  │
                  ▼
         primary: payments-api
         related: webhook-worker
        evidence: internal/webhooks/retry.go
```

## Install

### macOS and Linux

```bash
curl -fsSL https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.sh | sh
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.ps1 | iex
```

The installers download the matching release asset and verify its SHA-256 checksum. No language runtime is installed or required. You can also download a binary directly from [GitHub Releases](https://github.com/smallshellctw/whichrepo/releases) or run the GHCR image.

Developers building from source can use `make build`; end users do not need Go. See the complete [installation guide](docs/install.md).

## 30-second start

Point WhichRepo at the directory that contains your services:

```bash
whichrepo init ~/code/commerce-platform
cd ~/code/commerce-platform
whichrepo "Add retry limits to checkout webhooks"
```

Open the local signal-map dashboard:

```bash
whichrepo dashboard --open
```

Refresh after repositories or manifests change:

```bash
whichrepo index
```

The SQLite index lives in the OS cache directory and is managed automatically. There is no database to install or keep running.

## Why WhichRepo?

- **Microservice-aware** — identifies the primary repository and related services instead of returning one fuzzy match.
- **Language-agnostic** — understands package manifests, workspace references, imports, and syntax across mixed-language systems.
- **Evidence-backed** — returns matching paths, symbols, aliases, package identities, and dependency signals.
- **Local by default** — SQLite + BM25, no telemetry, no source upload, and no required API key.
- **Agent-native** — stdio MCP and a bundled skill for Codex, Claude Code, Cursor, OpenCode, and Gemini CLI.
- **One binary** — CLI, MCP server, language analyzers, SQLite, and the React dashboard ship together.
- **Optional AI decision layer** — Jev via Vercel, TypeSafe, or OpenRouter with classified errors and local fallback.

## Language support

| Ecosystem | Project discovery | Dependency extraction | Syntax-aware evidence |
| --- | --- | --- | --- |
| Go | `go.mod`, `go.work` | modules + imports | Tree-sitter |
| JavaScript / TypeScript | `package.json`, npm/pnpm/yarn workspaces | packages + imports | Tree-sitter |
| Python | `pyproject.toml`, `requirements.txt` | distributions + imports | Tree-sitter |
| Java / Kotlin | Maven and Gradle | modules + dependencies + imports | Tree-sitter for Java |
| Rust | `Cargo.toml`, Cargo workspaces | crates + `use` | Tree-sitter |
| .NET | `.sln`, `.csproj`, `.fsproj`, `.vbproj` | project/package references | Tree-sitter for C# |
| PHP | `composer.json` | Composer packages + namespaces | Tree-sitter |
| Ruby | `Gemfile`, gemspec | gems + `require` | Tree-sitter |

Tree-sitter runs through a pure-Go runtime, so release binaries remain static and cross-platform with `CGO_ENABLED=0`. Unsupported files still participate in local text retrieval. See [language support](docs/languages.md) and the [analyzer extension contract](docs/analyzers.md).

## Coding-agent integration

MCP tools: `route_task`, `refresh_index`, `list_projects`, and `workspace_status`.

Codex:

```bash
codex mcp add whichrepo -- whichrepo mcp --workspace /path/to/workspace
```

Claude Code:

```bash
claude mcp add whichrepo --scope user -- whichrepo mcp --workspace /path/to/workspace
```

Cursor (`~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "whichrepo": {
      "command": "/absolute/path/to/whichrepo",
      "args": ["mcp", "--workspace", "/path/to/workspace"]
    }
  }
}
```

Restart the client after adding the server. Client-specific examples are in [`docs/integrations`](docs/integrations/).

## Dashboard

The embedded Dashboard binds only to `127.0.0.1` and includes:

- **Router** — task-to-repository signal map and evidence.
- **Repositories** — languages, package identities, dependencies, and index coverage.
- **Evaluation** — reproducible Top-1/Top-3 routing benchmarks.
- **Settings & Privacy** — provider state and the exact remote payload boundary.

CLI, Dashboard, and MCP use the same routing engine and result schema.

## Configuration

Commit shared metadata as `.whichrepo.yaml`:

```yaml
version: 1
workspace:
  name: commerce-platform
  exclude: [archive-*, tmp-*]
projects:
  payments-api:
    path: services/payments-api
    description: Checkout, payments, and webhook endpoints
    aliases: [checkout, payments, webhooks]
```

Keep personal or private aliases in `.whichrepo.local.yaml`, which should remain untracked.

| Environment variable | Purpose |
| --- | --- |
| `WHICHREPO_WORKSPACE` | Workspace root |
| `WHICHREPO_DB` | SQLite index path |
| `WHICHREPO_CONFIG` | Explicit configuration path |
| `WHICHREPO_PROVIDER` | `local`, `jev-vercel`, `jev-typesafe`, or `jev-openrouter` |
| `WHICHREPO_PROVIDER_URL` | Override the provider endpoint |
| `WHICHREPO_MODEL` | Override the provider model |

## Optional Jev routing

Local routing is the default. Jev is an optional re-ranker:

```bash
export WHICHREPO_PROVIDER=jev-vercel
export AI_GATEWAY_API_KEY="..."
whichrepo "Add retry limits to checkout webhooks"
```

Authentication, billing, rate-limit, timeout, and availability failures are reported separately and fall back to local results. Remote providers receive task text, candidate summaries, and redacted evidence—not complete source files.

## Reproducible evaluation

The included fixture contains Go, TypeScript, Python, Java, Rust, C#, PHP, and Ruby services:

```bash
whichrepo index --workspace ./examples/polyrepo \
  --config ./examples/polyrepo/.whichrepo.yaml \
  --db /tmp/whichrepo-demo.db

whichrepo eval --workspace ./examples/polyrepo \
  --config ./examples/polyrepo/.whichrepo.yaml \
  --db /tmp/whichrepo-demo.db \
  --dataset benchmarks/starter.jsonl
```

Current 100-task local baseline: **79% Top-1, 95% Top-3**. The benchmark and every fixture are public and run without a model API.

## Privacy

- Source files and the SQLite index stay on the local machine.
- Known credentials, environment files, private keys, generated code, and dependency directories are excluded.
- There is no default telemetry.
- Routing evidence is not authorization to edit, commit, deploy, or access production systems.

See [SECURITY.md](SECURITY.md) for the threat model.

## Development

Only contributors need Go and Node.js:

```bash
make test
make web
make build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [ROADMAP.md](ROADMAP.md).

## License

MIT
