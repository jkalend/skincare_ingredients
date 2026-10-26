# skincare_ingredients

Personal, fully-free skincare ingredient index. Takes a barcode, a label photo,
a pasted INCI list, or a URL; returns the ingredient list, its canonical INCI
names, what the EU regulatory annexes say about each, and where the data came
from. Built to be driven by an agent, not by a human at a terminal.

**Status: research complete, decisions recorded, no code.** Started 2026-10-26.
Language: Go 1.26 (ADR 0001). The claim-by-claim review and the cut v1 scope are
in [`docs/review-2026-10-26.md`](docs/review-2026-10-26.md). Next slice: port
the validated parser, then ship `lookup` / `parse` / `analyze` / `shelf`.

**Repo:** <https://github.com/jkalend/skincare_ingredients> (public). **CI:**
`workflow_dispatch` only — see [`docs/run-ci.md`](docs/run-ci.md) for running it
and having ChatGPT read the result.

## Hard constraints

| Constraint | Consequence |
|---|---|
| **No paid data, no API keys, no card** | Rules out INCIDB Complete ($79), Dermalytics, inciAPI, skincareapi.dev, SkinSort API. Free, keyless services (GitHub Actions, the CosIng Checker mirror) are allowed as CI or bootstrap, never as a required paid dependency. Core data comes from Open Beauty Facts, CosIng, scraping, OCR, or a local model. |
| **Agent-native** | Every operation is a non-interactive command with structured JSON output. The agent can also hand work *back* (URLs, decisions, unresolved tokens) instead of failing. |
| **Provenance or it didn't happen** | Every stored value carries `source`, `url`, `fetched_at`, `method`, `confidence`. `NULL` beats a guess. |
| **Deterministic before generative** | A fixed resolver chain handles tokens. A local LLM is the last resort, recorded as such, and its output is reviewable. |
| **Concentrations are never inferred** | Label order is the only signal and it is not a percentage. Anything numeric must be printed on the pack or it stays `null`. |

## Documents

| Doc | Contents |
|---|---|
| [`docs/research.md`](docs/research.md) | Source inventory with verification status as of 2026-10-26, what each licence allows, the hard limits of INCI data, and how ChatGPT can consume the result. |
| [`docs/review-2026-10-26.md`](docs/review-2026-10-26.md) | Claim-by-claim re-validation, corrections, v1 keep/extend/cut, storage location, free CI/CD. |
| [`docs/scraping.md`](docs/scraping.md) | Which sites are usable, verified `robots.txt` findings, the extraction ladder, the provenance record, and the consent rules. |
| [`docs/agent-interface.md`](docs/agent-interface.md) | CLI and MCP contract, job protocol, work orders, on-disk data model, multi-agent coordination. |
| [`docs/decisions/0001-language.md`](docs/decisions/0001-language.md) | Go 1.26 static binary; Python parser used only as an offline oracle. |
| [`docs/decisions/0002-data-and-ci.md`](docs/decisions/0002-data-and-ci.md) | Public repo, `data/` open data, `local/` private overlay, `workflow_dispatch`-only CI. |
| [`docs/run-ci.md`](docs/run-ci.md) | Plain-language ways to run CI and have ChatGPT read the result. |
| [`DATA-LICENSE.md`](DATA-LICENSE.md) | ODbL for OBF-derived data, CC BY 4.0 for CosIng-derived data, scraped data excluded. |
| [`AGENTS.md`](AGENTS.md) | Repo rules for any agent working in here. |

## Scope and decisions

- **v1, local and public-data:** `lookup`, `parse` (text/file/stdin), `analyze`,
  `resolve`, `shelf`, `export json|csv`, `doctor`. Deterministic resolver only;
  no model in the loop.
- **Deferred:** job files/locks/queue, OCR, local LLM, remote MCP, SQLite
  export, batch scraping, multi-agent coordination.
- **Language:** Go 1.26, one static binary. The MIT `on-the-list` parser is an
  offline porting oracle, not a runtime dependency.
- **Storage:** the repo is public. `data/` holds open, OBF/CosIng-derived data
  under ODbL/CC BY 4.0. `local/` is gitignored and holds the personal shelf and
  scraped observations. The OBF dump, raw HTML, images and HTTP cache are never
  committed.
- **CI:** public GitHub repo, **`workflow_dispatch` only** — no schedule, no
  push/PR trigger. Standard public-repo Actions minutes are free. Each run
  writes `ci/last-run.json` for ChatGPT and agents to read.

## What it will be able to do

- `lookup <barcode>` — best available ingredient list for a product, with confidence.
- `parse` — messy label text → ordered ingredient list, unresolved tokens kept.
- `analyze` — list → per-ingredient canonical name, CosIng function, annex status, unresolved tokens.
- `shelf` — what is on my shelf, which ingredients duplicate, which annexes they match.
- `export` — the same data as JSON or CSV, with provenance.
- `ingest <urls…>` — deferred until the local path is proven; scraping fallback, not the primary source.

## Licensing stance

- Code: TBD (MIT or Apache-2.0) once there is code.
- Data in `data/`: OBF-derived data is ODbL 1.0 (attribute Open Beauty Facts,
  share-alike); CosIng-derived data is CC BY 4.0 (attribute the European Union,
  indicate changes). See [`DATA-LICENSE.md`](DATA-LICENSE.md).
- CosIng data: © European Union, CC BY 4.0 (Commission Decision 2011/833/EU).
- Open Beauty Facts data: ODbL 1.0. **Share-alike**: a derived database published publicly must also be ODbL, with attribution to Open Beauty Facts.
- Scraped ingredient text: not ours to relicense. It lives in gitignored
  `local/` with provenance; it is never committed to the public repo.

## Non-goals

- Medical or dermatological advice; any "safe/unsafe" verdict.
- Concentration estimation, efficacy claims, routine recommendation.
- Beating INCIDecoder at being INCIDecoder. This indexes *your* shelf.
