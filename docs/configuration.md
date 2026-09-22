# Configuration

The only required input is the workspace directory. Pass it to `whichrepo init`, use `--workspace`, set `WHICHREPO_WORKSPACE`, or run WhichRepo from that directory.

## Precedence

Settings are applied from lowest to highest priority:

1. built-in safe defaults;
2. committed `.whichrepo.yaml`;
3. untracked `.whichrepo.local.yaml`;
4. `WHICHREPO_*` environment variables;
5. command-line flags.

This mirrors the layered configuration model used by mature developer CLIs: teams can commit shared metadata while individuals keep local paths and provider choices private.

## Discover the effective configuration

```bash
whichrepo config show
whichrepo config paths
whichrepo config defaults
whichrepo config validate
```

`whichrepo init /path/to/workspace` writes every optional setting with its default value and a YAML language-server schema comment. Editors that support YAML schemas provide validation and autocomplete from [`configs/whichrepo.schema.json`](../configs/whichrepo.schema.json).

## Full example

```yaml
version: 1
workspace:
  name: commerce-platform
  include: []
  exclude: [archive-*, tmp-*]

index:
  database: ""                 # Empty: OS cache directory
  max_files_per_project: 800
  max_bytes_per_file: 12288
  max_bytes_per_project: 4194304
  extra_extensions: []

routing:
  top_k: 8
  provider: local              # local, jev-vercel, jev-typesafe, jev-openrouter
  model: ""                    # Empty: provider default
  provider_url: ""             # Empty: provider default
  provider_timeout: 20s

dashboard:
  listen: 127.0.0.1:8787       # Loopback addresses only
  open: false

privacy:
  additional_exclude_dirs: []
  additional_sensitive_files: []

projects: {}
```

Built-in credential exclusions cannot be disabled through workspace configuration. The privacy lists only add exclusions.

## Environment variables

| Variable | Overrides |
| --- | --- |
| `WHICHREPO_WORKSPACE` | workspace directory |
| `WHICHREPO_CONFIG` | explicit configuration file; disables normal shared/local layering |
| `WHICHREPO_DB` | `index.database` |
| `WHICHREPO_PROVIDER` | `routing.provider` |
| `WHICHREPO_MODEL` | `routing.model` |
| `WHICHREPO_PROVIDER_URL` | `routing.provider_url` |
| `WHICHREPO_PROVIDER_TIMEOUT` | `routing.provider_timeout` |

Provider credentials are accepted only through `AI_GATEWAY_API_KEY`, `TYPESAFE_API_KEY`, or `OPENROUTER_API_KEY`. They are intentionally absent from the configuration schema.
