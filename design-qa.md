# WhichRepo Dashboard Design QA

- Source visual truth: `docs/design/reference-router.png`
- Implementation screenshot: `docs/design/implementation-router-v2.png`
- Responsive screenshot: `docs/design/implementation-router-responsive.png`
- Comparison board: `/private/tmp/whichrepo-design-compare-v2.png`
- Desktop viewport: `1440 x 1024` CSS pixels, device scale factor 1
- Source pixels: `1487 x 1058`, normalized to `1440 x 1024`
- Implementation pixels: `1440 x 1024`
- State: Router screen with the task “Add retry limits to checkout webhooks” and local routing results

## Full-view comparison

The implementation preserves the selected Signal Map composition: narrow left navigation, task command bar, large central multi-repository map, cyan analysis paths, lime primary repository, cyan related repositories, and a persistent evidence inspector. Visual density, graphite surfaces, typography hierarchy, and action placement match the source closely.

## Focused-region comparison

- **Signal map:** task, analysis, primary, related, and muted repository nodes use the same hierarchy and semantic colors as the source.
- **Evidence inspector:** three evidence groups, relevance values, local-only status, code-like excerpts, and the primary copy action match the source anatomy.
- **Navigation and input:** selected Router state, product identity, task input, and submit action remain legible at desktop and responsive widths.

## Required fidelity surfaces

- Typography: Inter and JetBrains Mono match the source intent; hierarchy and wrapping are readable.
- Spacing: desktop proportions and responsive stacking are consistent; no horizontal overflow at 700px.
- Colors: graphite, cyan, lime, slate, and restrained amber tokens match the selected direction with accessible contrast.
- Assets and icons: Phosphor icons and XYFlow graph primitives are used; no placeholder imagery or handcrafted SVG assets are present.
- Copy: primary product copy, sample task, repository names, privacy language, and CTA match the selected concept.

## Interaction verification

- Route task request completed against the local Go API.
- Router, Repositories, Evaluation, and Settings navigation worked.
- Copy-agent-context control is wired.
- Desktop and responsive layouts rendered without horizontal overflow.
- Browser console errors checked: none.

## Comparison history

### Iteration 1

- P1: Sidebar and oversized heading compressed the hero workflow.
- P1: Analysis nodes looked like generic process boxes instead of signal nodes.
- P2: Workspace graph appeared materially smaller than the source.

Fixes: narrowed the sidebar, reduced heading scale, introduced circular icon-based signal nodes, enlarged the task node, and rebalanced graph positions.

### Iteration 2

Post-fix comparison shows no actionable P0, P1, or P2 differences. The implementation intentionally adds Evaluation navigation and a provider-status pill because they are required product surfaces outside the source mock.

## Follow-up polish

- P3: Add subtle route-transition animation after the accessibility motion preference is implemented.

final result: passed
