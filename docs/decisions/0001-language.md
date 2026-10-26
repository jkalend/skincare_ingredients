# ADR 0001 — Language: Go

- **Status:** accepted
- **Date:** 2026-10-26
- **Supersedes:** none

## Context

The tool is an agent-driven, non-interactive CLI plus a local MCP server. It
runs on Windows for development and on Linux in GitHub Actions. The data path
parses a 17.9 MB Open Beauty Facts CSV and five CosIng annex CSVs; the query
path must start quickly and be cheap enough to invoke per agent command. The
user's constraint is the **fastest and slimmest build/run** that still stays
free.

The only reusable component is `on-the-list` (MIT, Python ≥ 3.11), whose parser
was independently reproduced at 16,535 / 16,635 labels (99.4 %) and 298,423
ingredients on the OBF corpus. It is also the main porting risk.

## Decision

Use **Go 1.26**, one static binary, `CGO_ENABLED=0`, no cgo, and no Python in
the product runtime. Tiny repo-tooling scripts (the docs link check and the CI
summary writer) may use the runner's Python; they do not ship, are not part of
the product build, and do not affect the Go binary.

Dependencies: standard library, `golang.org/x/text` (NFKC + case folding),
`golang.org/x/net/html` (HTML extraction), `github.com/modelcontextprotocol/go-sdk`
(official MCP SDK, v1.8.0, MCP spec 2026-07-28), `encoding/csv`, `encoding/json`.
No SQLite, no cgo, no browser for v1.

The `on-the-list` parser is ported to Go under its MIT licence. The Python
package is used only as an offline oracle while porting; it is not a build,
test or runtime dependency. The port is accepted only if it reproduces the
corpus baseline: 16,535 parsed labels and 298,423 ingredients, with the same
token boundaries on the pinned corpus.

## Consequences

- Measured on this machine: hello-world build 5.7 s (Windows) / 12.9 s (Linux
  cross, cold); the dependency set above produced an **8,085,666-byte** Linux
  binary in 39.9 s cold including toolchain and module download. Warm-cache
  builds are expected to be seconds `[INFERENCE]`.
- One language for the product (ETL, CLI, stdio MCP); the Python scripts are
  repo tooling only. Cross-compile Windows/Linux/macOS from one runner.
- Port cost: ~550 lines of parser/normalizer core plus golden tests. This is
  the only place where the validated Python behaviour can be lost.
- No PyInstaller/Nuitka, no venv, no runtime install; the binary is the
  deployment.
- If the port cannot meet the corpus gate, do not ship a hybrid Go/Python
  binary. Switch to Python 3.13 + `uv` (ADR amendment) and accept the larger
  artifact.

## Alternatives

| Option | Why not chosen |
|---|---|
| Python 3.13 + `uv` | Direct parser reuse and no compile, but no slim artifact; startup and dependency install are worse for a per-command agent tool. Viable fallback. |
| Rust | Smallest binary and fastest runtime, but slower CI cold builds and slower iteration for a data-quality project. |
| TypeScript/Bun | MCP SDK is first-class, but compiled binaries are large and OCR/ML tooling is weaker. Bun was not installed here. |
| C#/.NET AOT | Good runtime, but no advantage for this workload and more Windows/Linux build surface. |
