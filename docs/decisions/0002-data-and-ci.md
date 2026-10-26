# ADR 0002 — Data location, publication and CI

- **Status:** accepted
- **Date:** 2026-10-26
- **Supersedes:** none

## Context

The inventory is personal and small. The derived data combines OBF records
(ODbL, share-alike), CosIng annexes (CC BY 4.0) and, for products nobody has
recorded, scraped panels that are not ours to relicense. GitHub Actions is
free for public repositories and gives 2,000 minutes/month on the Free private
plan. GitHub Pages on Free is public-repository only; publishing from a private
repo needs Pro, and the site is public anyway.

## Decision

**Keep the repo private and commit the small derived data in `data/`.**

- `data/` is versioned: one JSON per product/ingredient, one shelf file, the
  registry CSV + content manifest, and the local indexes.
- `cache/` and `state/` are gitignored. The OBF dump (17.9 MB gzip CSV / 104 MB
  gzip JSONL), raw HTML, images, cookies and HTTP cache are never committed.
- JSON, not SQLite, is the source of truth for v1. No Git LFS.
- `data/` files are generated and idempotent: unchanged content must not
  rewrite a file or change `fetched_at`.
- The CosIng registry is identified by the **content hash of the CSV with the
  two export-metadata lines removed** plus the Commission's `last_update`.
  The raw CSV is kept for audit; the raw sha256 is not the registry identity.
- The query path is local: CLI plus stdio MCP. No GitHub Pages, no remote MCP,
  no Actions-as-API in v1.
- CI: private repo, `ubuntu-latest`, nightly `schedule` + `workflow_dispatch`,
  PR gate on a pinned 1,000-label fixture. Expected 2–5 minutes/run, i.e.
  60–150 minutes/month against the 2,000-minute Free allowance.

## Consequences

- No ODbL share-alike trigger while the repo is private; OBF is still
  attributed in the data and generated reports.
- Scraped panels stay local and unpublished, matching the repo rule.
- The shelf is not reachable from ChatGPT. That is acceptable for v1; the
  local agent can read `data/**` and speak MCP over stdio.
- If the repo is made public later, do not simply flip visibility. Split the
  code from the data, publish only OBF/CosIng-derived data under ODbL/CC BY 4.0
  with attribution, and keep shelf/scraped observations private.
- If ChatGPT access is wanted later, publish a read-only, non-personal subset
  to a free public host (e.g. Cloudflare Workers free, no card) or use Secure
  MCP Tunnel; do not expose write tools.

## Alternatives

| Option | Why not chosen |
|---|---|
| Public repo | Free unlimited Actions and Pages, but publishing OBF-derived data triggers ODbL share-alike and scraped panels may not be redistributable. |
| Private repo + Pages | Pages from a private repo needs Pro; site is public anyway. |
| Release asset / artifact as storage | Artifacts expire (500 MB private quota); release assets are not a query API. |
| Local Task Scheduler only | Zero CI minutes, but no reproducible history and no PR gate. |
