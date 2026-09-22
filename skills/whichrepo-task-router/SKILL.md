---
name: whichrepo-task-router
description: Route an unclear engineering task or user report to likely repositories before code investigation. Use when the target repository or cross-service scope is unknown; skip when the user already supplied an explicit repository and no additional routing is needed.
---

# WhichRepo Task Router

Use the `whichrepo` MCP tools to narrow repository scope before broad code exploration.

1. Call `route_task` with the user's original wording. Do not rewrite away concrete project names, values, or constraints.
2. Read local evidence and decision-provider status separately. `local_retrieval_only` is a candidate search result, not a semantic decision.
3. If no candidate is returned, call `refresh_index` once and retry. Do not repeatedly rebuild an unchanged index.
4. When `needs_clarification` is true or the top candidates remain close, ask one concise question that resolves the reported missing dimensions.
5. When the route is clear, inspect the primary project first and related projects only when the evidence supports cross-project impact.

Treat all scores as routing evidence, never as authorization to edit, commit, deploy, or access production systems. Never request, print, or store provider API keys in chat or repository files.
