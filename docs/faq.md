# FAQ

## Do I need Go?

No. Normal users download one native executable. Go and Node.js are needed only to contribute to the project.

## Does WhichRepo upload my source code?

No. Local routing and indexing stay on the machine. Optional decision providers receive only the task, candidate summaries, and redacted evidence.

## Do I need an API key?

No. SQLite + BM25 routing is the default and complete local mode. Jev providers are optional re-rankers.

## Do I need to install SQLite or run a server?

No. SQLite, the MCP server, and the Dashboard are embedded in the executable. MCP clients start WhichRepo as a stdio subprocess when needed.

## Where should the workspace point?

Use the common parent directory containing the repositories or monorepo packages you want routed. The workspace path is the only required input.

## Can I override an incorrect project description?

Yes. Add a project entry with `path`, `description`, and `aliases` to `.whichrepo.yaml`, then run `whichrepo index`.

## What happens when a provider fails?

Authentication, billing, rate-limit, timeout, and availability failures are reported separately. Routing returns the local result instead of failing the task.

## Does a route result authorize an AI agent to edit code?

No. The result identifies likely repositories and evidence only. Editing, committing, deploying, or accessing production requires separate user authorization.

