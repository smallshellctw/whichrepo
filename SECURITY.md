# Security Policy

WhichRepo indexes local project metadata and selected source prefixes into a local SQLite file. It does not collect telemetry. Optional decision providers receive only task text, candidate summaries, and redacted evidence.

Security fixes are provided for the latest release. Report vulnerabilities privately through GitHub Security Advisories for `smallshellctw/whichrepo`; do not open a public issue for source or credential exposure.

Expected protections:

- Credential-like files are excluded before indexing.
- Provider API keys are read from environment variables and never returned by CLI, MCP, or Dashboard APIs.
- The Dashboard only binds to loopback addresses.
- Remote-provider failure always falls back to local routing.
