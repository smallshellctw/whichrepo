# Benchmark methodology

The public starter benchmark contains 100 engineering tasks across a synthetic 11-project microservice workspace using Go, TypeScript, Python, Java, Rust, C#, PHP, Ruby, and infrastructure metadata.

## Metrics

- **Top-1:** at least one expected project is the first candidate.
- **Top-3:** at least one expected project appears in the first three candidates.
- Multi-project tasks may declare more than one expected project.

Run the benchmark without a model API:

```bash
whichrepo index --workspace ./examples/polyrepo \
  --config ./examples/polyrepo/.whichrepo.yaml \
  --db /tmp/whichrepo-benchmark.db

whichrepo eval --workspace ./examples/polyrepo \
  --config ./examples/polyrepo/.whichrepo.yaml \
  --db /tmp/whichrepo-benchmark.db \
  --dataset ./benchmarks/starter.jsonl
```

Current local baseline: 79% Top-1 and 95% Top-3.

## Limits

- The fixture is synthetic and cannot represent every naming convention.
- Results measure repository selection, not code-generation quality.
- Public tasks intentionally avoid proprietary code and company-specific vocabulary.
- Provider comparisons must use the same index, dataset, and candidate limit.

Contributions should include the dataset change and before/after results. Do not submit private task text or unverifiable accuracy claims.

