# Distribution staging

The initial public release is runtime-free and distributed through GitHub Releases, the install scripts, and GHCR.

External registries are intentionally staged but not published yet:

- `homebrew/whichrepo.rb.tmpl` — Homebrew tap formula template;
- `scoop/whichrepo.json.tmpl` — Scoop manifest template;
- `winget/` — WinGet manifest notes;
- npm/PyPI discovery packages — deferred until package ownership is reserved.

Release automation must replace version and checksum placeholders from the exact GitHub Release artifacts. Never publish a manifest pointing at an unverified or mutable binary.

