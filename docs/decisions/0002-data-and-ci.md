# ADR 0002 — Public data, local overlay, on-demand CI

- **Status:** accepted; amended 2026-10-26
- **Date:** 2026-10-26
- **Supersedes:** the earlier private-repo/nightly version of this ADR

## Context

The inventory is personal but the repo is now **public**, and CI runs **only
when invoked** — no `schedule`, no nightly job, no push/PR trigger.

That changes the licensing and privacy shape:

- Open Beauty Facts data is ODbL 1.0. A public derived database must also be
  ODbL, with attribution.
- CosIng annexes are CC BY 4.0; attribution and changes indicated.
- Raw scraped ingredient panels are not ours to relicense. Captures cannot go
  into a public repo.
- The shelf is personal. It can be public if the owner puts it in `data/`, but
  the safe default is a gitignored local overlay. A normalized product/formula
  catalog can be published only after explicit owner authorization and a
  source-terms check.
- GitHub Actions is free and unlimited for standard runners on public repos.

## Decision

**Public repo, open data in `data/`, personal/scraped data in `local/`.**

- `data/` is committed and public: products, ingredients, registry CSVs +
  manifest, indexes, and optional public shelf. OBF-derived data is ODbL 1.0;
  CosIng-derived data is CC BY 4.0. `DATA-LICENSE.md` states both and the
  attribution.
- `local/` is gitignored: the personal shelf by default, scraped observations,
  raw HTML, images, cookies and anything whose source terms forbid
  redistribution. `data/catalog/products.json` is an explicit owner-authorized
  exception containing normalized product names, ingredient tokens, source
  links and uncertainty; it excludes ownership counts, wishlist entries,
  personal notes and raw captures.
- If the owner wants the shelf public, move `local/shelf/` to `data/shelf/`.
  That is an explicit publication choice, not a default.
- CI is **`workflow_dispatch` only**. No `schedule`, no `push`, no
  `pull_request`. The workflow is started from the GitHub UI, `gh workflow run`,
  or a ChatGPT Action/MCP that calls the GitHub API.
- Every run writes `ci/last-run.json` to `main`: status, conclusion, commit,
  run URL, check outcomes and a short error tail. That file is the machine-
  readable result ChatGPT or an agent reads after the run.
- The query path is CLI plus stdio MCP. A **read-only Streamable HTTP MCP**
  endpoint exposes public data for ChatGPT; it omits shelf tools, does not load
  `local/`, and can require a bearer token. A read-only Pages/JSON view is
  optional later.
- No remote MCP write endpoint. ChatGPT gets a CI trigger tool only if the owner
  adds a Custom GPT Action or MCP that holds a fine-grained GitHub token.

## Consequences

- ODbL share-alike applies to the public catalog database. Attribute Open
  Beauty Facts and keep its derivative database ODbL. Do not publish scraped
  panel captures or source material whose terms forbid redistribution.
- The public repo can be read without a token. Triggering a workflow still
  needs an authenticated GitHub token with `actions: write`.
- No schedule means no 60-day auto-disable problem and no wasted minutes.
  Public-repo Actions minutes are free; the only cost is the run you asked for.
- The shelf is private by default. ChatGPT can read public `data/` and
  `ci/last-run.json`; it cannot read `local/` unless the owner publishes it.
- Bot commits from the workflow update `ci/last-run.json`; a push made with
  `GITHUB_TOKEN` does not trigger another workflow.

## Alternatives

| Option | Why not chosen |
|---|---|
| Private repo | 2,000 Actions min/month and no ODbL trigger, but the user asked for public and a public repo is unlimited/free. |
| Public repo with private shelf in a separate private repo | More moving parts; a gitignored `local/` overlay is enough. |
| Nightly `schedule` | Rejected by the user; also delayed under load and auto-disabled after 60 days of inactivity in public repos. |
| Push/PR triggers | Rejected: runs must happen only on invocation. |
| Remote MCP write endpoint | Needs a public host and a Business/Enterprise ChatGPT plan for full MCP; the token belongs in a narrow Action/proxy, not a chat. |
