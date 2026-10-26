# AGENTS.md

Repo purpose: a free, agent-driven skincare ingredient index. Read
[`docs/research.md`](docs/research.md) before proposing anything; read
[`docs/agent-interface.md`](docs/agent-interface.md) before designing a command.

**Current phase: foundations.** The language (Go 1.26) and the public/on-demand
CI design are recorded in `docs/decisions/`. There is no product code yet; start
from the first slice in [`docs/review-2026-10-26.md`](docs/review-2026-10-26.md)
§8.

## Must

- Keep every dependency, data source and runtime **free**. No paid API, no
  trial key, no "free tier with a card".
- Give every persisted ingredient value provenance: `source`, `url`,
  `fetched_at`, `method`, `confidence`. Missing value → `null`, never a guess.
- Run the deterministic resolver chain before any model call. Model output is
  stored with `method: "llm"`, the model name, and a confidence, and is never
  silently promoted to fact.
- Store unresolved tokens verbatim and flag them. They are data, not errors.
- Regenerate generated files; never hand-edit them.
- Honor `robots.txt` for the user agent actually in use, plus the per-host
  request budget in [`docs/scraping.md`](docs/scraping.md).
- Keep `data/` to open data: OBF-derived data is ODbL, CosIng-derived data is
  CC BY 4.0. Attribution and the database licence stay in
  [`DATA-LICENSE.md`](DATA-LICENSE.md).
- CI is invocation-only: `workflow_dispatch`. Do not add `schedule`, `push` or
  `pull_request` triggers without an explicit decision.

## Must not

- Infer, estimate or back-solve a concentration from label position.
- Emit a compliance, safety or efficacy verdict. Annex matches are reported as
  matches, with the annex and entry number.
- Bypass bot protection (Cloudflare interstitials, CAPTCHAs) or use a login to
  reach ingredient text.
- Commit scraped HTML, scraped ingredient panels, the personal shelf,
  credentials, cookies, or anything from a source whose terms forbid
  redistribution. Those live in gitignored `local/`.
- Present a single unconfirmed scrape as verified. One source = `unverified`.

## Conventions

- Docs in `docs/`, dated claims state the date they were checked.
- Unverified claims are marked `[UNVERIFIED]` inline, not smoothed over.
- Data model and command surface are contracts; change them in the doc first.
