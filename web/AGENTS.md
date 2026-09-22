# Prototype Instructions

Run the local server yourself and open the preview in the browser available to this environment. Do not give the user server-start instructions when you can run it.

Before making substantial visual changes, use the Product Design plugin's `get-context` skill when the visual source is unclear or no longer matches the current goal. When the user gives durable prototype-specific design feedback, preferences, or decisions, record them in `AGENTS.md`.

When implementing from a selected generated mock, treat that image as the source of truth for layout, component anatomy, density, spacing, color, typography, visible content, and hierarchy.

Build app UI in `src/`. Keep `.openai/hosting.json`, `worker/index.js`, `scripts/prepare-sites-build.mjs`, and `tests/sites-worker.test.mjs` intact so the same local prototype can be handed to Sites. Before a Sites handoff, run `npm run build` and `npm run test:sites`; the build must leave `dist/client/index.html`, `dist/server/index.js`, and `dist/.openai/hosting.json`.

## WhichRepo visual target

- Source of truth: `../docs/design/reference-router.png`.
- Emphasize multi-repository and microservice routing through a central signal map and a persistent evidence inspector.
- Use graphite surfaces, cyan analysis paths, electric-lime primary matches, Inter UI type, and JetBrains Mono for paths/code.
- Use Phosphor for interface icons and XYFlow for the repository graph; do not hand-draw icons or graph SVGs.
- Keep the Router as the primary screen. Repositories, Evaluation, and Settings may be supporting views, but must not crowd the hero workflow.
