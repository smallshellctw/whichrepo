# Language support

WhichRepo combines three independent signals:

1. package and workspace manifests;
2. imports and project references;
3. syntax-aware symbols extracted with Tree-sitter.

A syntax parser failure never prevents indexing. The manifest and bounded text index remain available as fallbacks. Files without a syntax parser still participate when their extension is part of the bounded source index.

| Ecosystem | Manifests | Source forms |
| --- | --- | --- |
| Go | `go.mod`, `go.work` | packages, imports, functions, types |
| JavaScript / TypeScript | `package.json`, npm/pnpm/yarn workspaces | imports, exports, functions, classes |
| Python | `pyproject.toml`, `requirements.txt` | imports, functions, classes |
| Java / Kotlin | `pom.xml`, `build.gradle`, `build.gradle.kts` | Java imports, methods, classes, inheritance |
| Rust | `Cargo.toml` | crates, `use`, functions and types |
| .NET | `.sln`, `.csproj`, `.fsproj`, `.vbproj` | project/package references and C# symbols |
| PHP | `composer.json` | packages, namespaces, classes and functions |
| Ruby | `Gemfile`, `*.gemspec` | gems, `require`, classes and methods |

## Why the parser does not require CGO

WhichRepo uses `github.com/odvcencio/gotreesitter`, a pure-Go implementation of the Tree-sitter runtime. Release builds include only the supported grammar blobs and are compiled with `CGO_ENABLED=0`.

Consequences:

- a single executable on macOS, Windows, and Linux;
- no C compiler on contributor or user machines;
- no runtime grammar downloads;
- deterministic, offline indexing;
- normal Go cross-compilation and race-test visibility.

The scanner reads only bounded prefixes of supported text files. Dependency directories, generated files, credentials, environment files, private keys, and ignored Git files are excluded.
