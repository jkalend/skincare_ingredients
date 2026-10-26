# Research — free, agent-native skincare ingredient index

Checked 2026-10-26. Every status cell below was observed by direct request from
this machine on that date; anything not observed is marked `[UNVERIFIED]`.

> **Re-validated 2026-10-26:** claim-by-claim results and corrections are in
> [`review-2026-10-26.md`](review-2026-10-26.md). The material updates: OBF is
> back up (the 502s were transient); CosIng Checker's OpenAPI is at
> `/api/v1/openapi.json`; raw annex CSV hashes change only in the two
> export-metadata lines. Use live CosIng CSVs for annexes, the mirror only for
> names/functions.

## 1. Verdict

The four layers exist as free components. The new code is glue: a resolver, a
cache, a scraper, and a thin command surface an agent can drive.

- **Data**: Open Beauty Facts (products, ODbL) + CosIng annexes (regulatory,
  CC BY 4.0) + a scraped fallback for products nobody has recorded.
- **Canonicalisation**: deterministic chain; a local model only for the residual
  tokens, always tagged.
- **Publish**: static JSON built by an on-demand `workflow_dispatch` run,
  committed to the public repo; optional Pages. Free for public repos.
- **Consume**: read-only MCP over the same data, which ChatGPT can attach as a
  custom connector; CLI covers everything else.

CI/CD belongs to the **data path** (ingest, canonicalise, verify, pin, publish).
It cannot serve the **query path**: Actions return `204`, need a PAT with
`actions:write`, and cold-start in tens of seconds to minutes.

## 2. Free sources

| Source | What it gives | Licence | Access | Verified 2026-10-26 |
|---|---|---|---|---|
| **Open Beauty Facts** | product → brand, barcode, raw `ingredients_text`, tag list | ODbL 1.0, share-alike | API v2/v3 + bulk dumps; same codebase as Open Food Facts | **200** on the site, CSV (17.9 MB gzip, 64,237 rows) and JSONL (104 MB gzip); API v2/v3 200 for a real barcode. The earlier 502 was a transient outage. Bulk access must use the dumps; the API is for one real scan at a time. |
| **CosIng annexes** (EU Commission) | Annex II–VI entries: prohibition/restriction, product types, max concentration, warnings, CAS/EC, `Identified INGREDIENTS` names | CC BY 4.0 (Commission Decision 2011/833/EU) | `GET https://api.tech.ec.europa.eu/cosing20/1.0/api/annexes/{II,III,IV,V,VI}/export-csv` | **200** for all five, `text/csv`. Reference-like rows: II 1,758; III 381; IV 154; V 58 (incl. `12 a`); VI 34. All say `Last update: 13/10/2026`, `File creation date: 26/10/2026`. Columns include `Identified INGREDIENTS or substances e.g.` — the join key back to printed label names. Raw bytes differ from the 2026-09-09 pin only in the two metadata lines; strip them before hashing. |
| **CosIng inventory + functions** | per-ingredient function list (83 functions), INN/Ph.Eur names, restrictions | same | **not exposed by the public API** (see §3) | 404/401 on every path tried |
| **CosIng Checker mirror** | the inventory above, searchable by name/CAS/EC/function; annex entries; EU INCI glossary 2025 | upstream licences vary per source; site data declared CC BY 4.0 | `https://cosingchecker.com/api/v1/`, no key, 120 req/min, OpenAPI 3.1 at `/api/v1/openapi.json` | **200**: `ingredients/?q=niacinamide` → `count: 18`; `functions/` → `count: 83`; 21 `/api/v1` path entries; stats: 28,712 ingredients, 30,418 glossary rows (EU 2025/1175), 2,353 annex records. Its annex counts are stale against the live CSVs (1,739 vs 1,758 for Annex II), so use it for names/functions only. Third-party mirror; acceptable as a bootstrap, not as a contract. |
| **on-the-list** (`Waiga/on-the-list`) | CLI: label text → annex findings; ships pinned annex CSVs with sha256 | MIT | PyPI, Python ≥3.11 | Package 0.1.4, register last update 28/08/2026, Annex II = 1758 entries; exit codes `0/1/2`. Corpus reproduced exactly: 16,535 / 16,635 labels, 298,423 ingredients. Its pinned annex rows are identical to the live 2026-10-26 content after stripping export metadata. |
| **OCR** | panel photos → text | Apache-2.0 / BSD | Tesseract, PaddleOCR, or browser-side OCR | `[UNVERIFIED]` — not tested here; a sibling project (`sarahbachernegg/open-inci-lab`) does it in-browser. |
| **Local LLM** | residual token resolution only | per model | already on disk (RTX 4090) | n/a |

## 3. CosIng API surface, as probed

Working:

```
GET /cosing20/1.0/api/annexes/II/export-csv    200  CSV
GET /cosing20/1.0/api/annexes/IV/export-csv    200  CSV
```

Not available:

```
GET /cosing20/1.0/api/annexes                   401
GET /cosing20/1.0/api/ingredients               404
GET /cosing20/1.0/api/ingredients/search        404
GET /cosing20/1.0/api/ingredient/36535          404
GET /cosing20/1.0/api/substances                404
GET /cosing20/1.0/api/regulations               404
GET /cosing20/1.0/api/ingredients/export-csv    404
GET /v3/api-docs  | /api/v3/api-docs  | /swagger-ui/index.html   404
```

Consequence: **the annexes are free and authoritative; the inventory is not on
the documented API.** The CosIng SPA's advanced search calls an internal
`webgate.ec.europa.eu/es/search-api` with an embedded key; that is an
implementation detail, not a contract, and the official inventory endpoint is
still absent.

The free workaround is the CosIng Checker mirror: 28,712 ingredients, 30,418
glossary rows and 83 functions, no key, 120 req/min, CC BY 4.0. It is
third-party and its annex snapshot is stale (1,739 Annex II records vs the live
1,758), so use it for **names and functions only**. Store
`source: cosingchecker`, `method: mirror`, a snapshot date and a confidence;
never silently promote a mirror value to fact.

The EU INCI glossary (Commission Decision (EU) 2025/1175) is the authoritative
name list. The mirror exposes it with `source_version: EU 2025/1175` and
`source_url: https://eur-lex.europa.eu/eli/dec_impl/2025/1175`, `applies_from:
2026-07-30`. It was not independently parsed from EUR-Lex here — `[UNVERIFIED]`
at source.

**Registry hashing:** the raw CSV sha256 is not a stable identity. Across all
five annexes, the live file and the 2026-09-09 `on-the-list` pin differ only in
the two export-metadata lines; after dropping those lines the bytes are
identical. Hash the data rows (or the CSV without metadata), store
`last_update` separately, and keep the raw file for audit.

## 4. Paid, therefore excluded

| Option | Price | Why excluded |
|---|---|---|
| INCIDB Complete | $79 one-time | Paid. Its **free sample** (200 products, 1,123 ingredients, ODbL) is usable as a *test fixture* — products were selected for ≥80 % CosIng match, so it is biased toward clean data. |
| Dermalytics | credits | Paid above 5 requests/IP/day; hosted MCP needs a key. |
| inciAPI | 2 000 free then paid | Paid. |
| skincareapi.dev | paid | Paid. |
| SkinSort API | paid | Paid. |
| Vinkius OBF MCP | hosted free tier | Free but no control over freshness; OBF was down today. |

Worth keeping from INCIDB as design input, not data: its coverage honesty table
(CosIng matches **12.2 % of distinct names but 85.4 % of label occurrences**) and
its per-row `method` + `confidence` discipline on the raw-token → canonical map.

## 5. What an ingredient list cannot tell you

These are the reasons to own the data instead of asking a model each time.

1. **No percentages.** Art. 19(1)(g) of Regulation (EC) No 1223/2009 requires
   descending order of weight, and **ingredients below 1 % may be listed in any
   order**. Position is an ordering fact, never a number.
2. **"Actives" is not a field.** The nearest machine-readable proxy is CosIng's
   function list (skin conditioning, humectant, antioxidant, UV filter,
   preservative…). Those are regulatory descriptors, not benefits. "Brightening"
   is editorial, i.e. our own curated layer or nobody's.
3. **Annexes are not an inventory.** Absence from Annexes II–VI means *these
   annexes say nothing about it* — not "invalid INCI name", not "safe".
   `Aqua` and `Glycerin` appear in none of them.
4. **Matching is the trap.** In `on-the-list`'s audit of 16,635 real labels, one
   over-broad mapping produced 99 of 1,066 Annex II findings: entry 1388 names
   `D4`, whose "identified" name is `CYCLOMETHICONE` — a mixture. Exact-name
   joins only; no fuzzy matching; fragments never match.
5. **The label is dirty.** Same corpus: 25.8 % of ingredient fields contained
   marketing prose, 9.8 % carried a section heading.
6. **Jurisdiction.** EU ≠ US ≠ KR. One dataset, one set of annexes, stated
   explicitly in every report.

## 6. Parsing, measured on real labels

From `on-the-list`'s pinned corpus (Open Beauty Facts export, records with
`ingredients_text` ≥ 50 chars):

| | |
|---|---|
| Records in export | 64,237 |
| Labels selected | 16,635 |
| Labels whose list parsed | 16,535 |
| Ingredients parsed | 298,423 |
| Annex II matches | 2,408 on 2,037 labels |
| Repeated entry | 712 on 291 labels |

So a hand-built parser gets ~99.4 % of real panels, and the remaining 0.6 % plus
the 25.8 % prose contamination is where the effort goes. Budget for the tail.

**Reproduced 2026-10-26** with `on-the-list` 0.1.4 over the live CSV: 64,237
records, 16,635 labels ≥ 50 chars, 16,535 parsed (99.4 %), 298,423 ingredients,
11.95 s single-threaded. Implementation detail: call `trim()` + `parse()` on
OBF `ingredients_text`; `extract()` requires an `Ingredients:` preamble and
finds only 782 labels (4.7 %). The Annex II match counts (2,408 / 2,037) and
repeated-entry count (712 / 291) were not independently re-derived —
`[UNVERIFIED]`. The Go port measures 16,545 / 16,645 labels and 298,436 tokens
(99.399 %); the 10-row difference from the Python run is CSV quote handling.

Resolver chain to copy (order matters, each step recorded per row):
`exact` → `cleaned` → `typo_map` → `percent_stripped` → `paren_stripped` →
`synonym_map` → `slash_variant` → `split` → `slash_same_cas` → `unresolved`.

## 7. Making ChatGPT (and other agents) consume it

| Option | Fit | Constraints, verified 2026-10-26 |
|---|---|---|
| **Remote MCP server** | best | ChatGPT connects to **remote** servers only (local needs Secure MCP Tunnel). **Pro: read/fetch permissions only; full MCP incl. write is Business/Enterprise/Edu.** Web only, not mobile. Tool list is frozen per publish — changes need an admin refresh. Deep research can use it read-only. |
| **Custom GPT Action** (OpenAPI 3.1) | fallback | `platform.openai.com/docs/actions` → 200, still current. Needs a public HTTPS host; keep it a thin proxy over the same JSON. |
| **Repo + agent** | zero infra | Codex/Claude Code read `data/**` directly. Loses barcode lookup for chat users. |

Design implication: expose **read tools publicly, writes behind a local CLI or an
authenticated path**. Nobody else's ChatGPT should be able to write to the shelf.

## 8. CI/CD: where it fits

| Job | Verdict |
|---|---|
| On-demand ETL: `workflow_dispatch` → OBF dump + CosIng annexes → parse → canonicalise → `data/**` JSON | yes |
| PR gate: parse-rate and Annex II count within tolerance of the pinned corpus | deferred; runs are invocation-only |
| Pin the regulatory register per build (stripped content hash + `last_update`) so every report is reproducible | yes |
| `workflow_dispatch` batch: OCR + parse a folder of panel photos, return an artifact | deferred (v2) |
| Answer a chat turn synchronously | no |
| Cron / nightly | **not used.** No `schedule`, no push/PR trigger. Public-repo Actions minutes are free; each run writes `ci/last-run.json`. |

## 9. Decisions (was: open questions)

1. **Language:** Go 1.26, one static binary; Python `on-the-list` is an offline
   oracle only. See [`decisions/0001-language.md`](decisions/0001-language.md).
2. **CosIng inventory:** use the live official CSVs for annexes; use the
   CosIng Checker mirror for the inventory/glossary/functions, snapshotted and
   tagged `method: mirror`. Revisit an official scrape only if the mirror
   disappears or drifts.
3. **OBF:** dump-first; verified available. API only for one real scan.
4. **OCR:** cut from v1; barcode + pasted text first.
5. **Publication:** public repo. `data/` is public, OBF-derived data is ODbL,
   CosIng-derived data is CC BY 4.0; scraped panels and the personal shelf stay
   in gitignored `local/` unless explicitly published. See
   [`decisions/0002-data-and-ci.md`](decisions/0002-data-and-ci.md).
