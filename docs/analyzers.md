# Language analyzer contract

Language analysis is isolated behind the public `github.com/smallshellctw/whichrepo/analyzer` package:

```go
type LanguageAnalyzer interface {
    Name() string
    Matches(path string) bool
    Analyze(path string, source []byte) Result
}
```

Every analyzer returns the same language-neutral result:

- language name;
- package or module identifiers;
- dependency identifiers;
- symbols;
- short evidence strings for local retrieval.

The SDK exposes `APIVersion`, `Result`, `LanguageAnalyzer`, `Registry`, and `NewRegistry`. The official binary registers manifest analyzers before syntax analyzers and finishes with a bounded-text fallback.

## Adding a built-in ecosystem

1. Add its manifest names to `ManifestNames`.
2. Parse only local, deterministic metadata.
3. Map source extensions in `LanguageForPath`.
4. Add a grammar only when syntax facts materially improve routing.
5. Add the grammar subset tag to `Makefile`, CI, and GoReleaser.
6. Add manifest, syntax, dependency-resolution, cross-build, and routing tests.
7. Document privacy implications and upstream grammar licensing.

Analyzer failures should degrade to bounded text retrieval. They must never abort the entire workspace index because one source file is malformed.

## Stability

`APIVersion` is currently `1`. Custom analyzers are linked into custom builds; WhichRepo deliberately avoids loading unsigned native shared libraries at runtime. An out-of-process protocol may be added later without changing the route result schema.
