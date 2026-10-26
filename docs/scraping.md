# Scraping

Products missing from Open Beauty Facts get their ingredient panel scraped.
Free, but not lawless: the cost of scraping badly is mostly that the data is
worthless, and secondarily that a host blocks us.

## 1. Consent rules

1. **Fetch `robots.txt` before the first request to a host in every run.** Cache
   it for the run; re-check daily. Record the decision and the UA used.
2. **If a host names AI agents explicitly, treat the host as excluded** — even
   when the generic `*` group would allow us. Renaming the UA to a generic
   string to slip under an explicit AI block is evasion, not compliance.
3. **Never** bypass an interstitial, CAPTCHA or JS challenge, never log in, never
   use a cached session from someone else.
4. **Budget per host:** max 1 request / 2 s, max 50 pages per run, stop the host
   for the run on `403`, `429` or a challenge page. Honour `Retry-After`.
5. **Identify the client:** `SkincareIngredients/0.1 (+<repo url>; personal
   research; contact <email>)`. No browser spoofing.
6. **Cache first.** A source younger than 30 days is not refetched unless
   `--refresh`. Conditional requests (`If-None-Match` / `If-Modified-Since`) when
   available.
7. `robots.txt` is not the terms of service. Where terms forbid automated
   collection or redistribution, the data stays local and is never published.
8. **Open Beauty Facts is not a scraping target.** Use the nightly CSV/JSONL
   dumps for bulk work; the API is for one real user scan at a time. The OBF
   data page states that bulk API use will be blocked and that dumps exist for
   this reason.

## 2. Hosts, verified 2026-10-26

Rechecked on 2026-10-26; the runtime check remains the authority.

| Host | `robots.txt` as served | Verdict |
|---|---|---|
| `incidecoder.com` (→ `inkeedecoder.com`) | 301 to `inkeedecoder.com`; `User-agent: *` → `Allow: /`, `Disallow: /auth/`, `Disallow: /products/recommend/`; sitemap at `inkeedecoder.com/sitemap-index.xml` | **useful** — product pages allowed; site is the best-known INCI panel layout |
| `skinsort.com` | One combined group for `GPTBot`, `ClaudeBot`, `PerplexityBot`, `CCBot`, `Bytespider`, `Google-Extended`, `Meta-ExternalAgent`, `Amazonbot`, `Applebot-Extended` disallows `/products/`, `/brands/`, `/dupes/`, `/analysis/`, `/compare/`, `/ingredients/`, `/feed_items` and the account paths. The file now explicitly says AI *search/user-directed* fetchers (`OAI-SearchBot`, `ChatGPT-User`, `Claude-SearchBot`, `Claude-User`, `Meta-ExternalFetcher`) use the `*` group, which has additional disallows (`/products/*/attributes`, `/compare/ingredients/`, `/shelf/`, …) | **caution** — legal for a generic UA, but the named groups state intent. Prefer as fallback; never at scale |
| `incibeauty.com` | `ChatGPT-User`, `GPTBot`, `OAI-SearchBot`, `ClaudeBot`, `Claude-User`, `Claude-SearchBot`, `PerplexityBot`, `Perplexity-User`, `MistralAI-User`, `Applebot`, `DuckAssistBot`, … each `Disallow: /`; `*` → `Disallow:` (allow) | **excluded** — explicit anti-AI stance |
| `cosdna.com` | 403 Cloudflare `Just a moment...` interstitial instead of `robots.txt` | **excluded** — protected, no legitimate path |
| brand/retailer sites | per host | **primary target** — a brand's own product page is the most authoritative and the least contested source |

Robots files change. The table is evidence of what was seen, not a permanent
permission slip — the runtime check is the authority.

## 3. Extraction ladder

First fit wins; each step records which one produced the text.

| # | Step | Notes |
|---|---|---|
| 1 | Structured data — JSON-LD `Product`, microdata, RDFa | Rarely has INCI; `description` often does |
| 2 | Embedded app state — `__NEXT_DATA__`, `__NUXT__`, `__APOLLO_STATE__`, `window.initialState` | Walk all string values, score by INCI markers |
| 3 | DOM around a heading — `h1..h4`/`dt`/`strong` matching `/ingredients?|inci|composition|složení/i`, plus `[id*=ingredient]`, `[class*=ingredient]` | Covers most brand sites |
| 4 | Whole-page text heuristic — longest comma-separated run containing `aqua|water` and ≥ 5 tokens that resolve against CosIng | Catches accordion/`details` layouts |
| 5 | Image OCR — product/back-of-pack images (`tesseract`/`paddleocr`, `hrv`-safe language packs) | Only when text is absent |
| 6 | Local LLM span extraction over the raw text | **Must return exact character offsets**, stored with the source substring. Never paraphrased, never reordered |

## 4. Validation gates

A candidate list is rejected unless all hold:

- ≥ 3 comma-separated tokens after splitting on `,` / `;` / `•` / newline.
- Contains `aqua`, `water` or an anhydrous marker (e.g. `balm`, `oil`, `stick`)
  — otherwise the panel is missing or truncated.
- ≥ 60 % of tokens resolve through the deterministic chain (see
  [`research.md`](research.md) §6); below that, store as `unresolved_document`.
- Contains no sentence-level punctuation runs (`Directions`, `Apply`, `Warning`,
  `Shake well`) — those indicate prose contamination.
- Never contains `may contain` block mixed into the main order; split it into a
  separate `may_contain[]` field, since its order carries no information.

## 5. Trust and consensus

Group observations by product identity: `barcode` if present, else normalised
`brand + name + size`.

| Observations | `trust` |
|---|---|
| ≥ 2 independent hosts, ≥ 90 % identical token sequence | `verified` |
| 2 hosts, 70–90 % identical | `probable` (store both, diff stored) |
| 1 host | `unverified` |
| 2 hosts, conflicting panels | `conflict` — both kept, nothing chosen silently |
| OCR only | `unverified` + `method: ocr` |

The manual label wins over everything if a human or agent supplies one:
`trust: manual`, `source: label-photo`.

## 6. Provenance record

```json
{
  "id": "sha256:9f2c…",
  "product_key": "ean:3600542134567",
  "url": "https://brand.example/p/cream-50ml",
  "host": "brand.example",
  "fetched_at": "2026-10-26T14:02:11Z",
  "http_status": 200,
  "body_sha256": "…",
  "extractor": "dom_heading|jsonld|state|text_heuristic|ocr|llm_span",
  "extractor_version": "0.1.0",
  "method": "exact|cleaned|…|llm",
  "confidence": 0.0,
  "raw_text": "Aqua, Glycerin, …",
  "robots_allowed": true,
  "user_agent": "SkincareIngredients/0.1 (+…)",
  "notes": []
}
```

Stored: extracted text + hash + metadata. **Not** stored: full HTML, images
beyond what OCR needs, cookies, anything from behind a login.

## 7. Handing work back

Some fetches cannot be completed by the tool and should not be. Each becomes a
work order for the calling agent (see
[`agent-interface.md`](agent-interface.md) §4) rather than a hard failure:

| Situation | Work order |
|---|---|
| Host excluded by `robots.txt` / anti-AI group | `kind: fetch_url`, `suggested: [alternative_hosts…]` |
| Challenge page | `kind: fetch_url`, `reason: bot_protection` — agent may find a brand page instead |
| Product ambiguous (same name, different sizes) | `kind: decide`, options with URLs |
| Panel found but 3 tokens unresolved | `kind: resolve_token`, tokens + nearby context |
| No panel on any source | `kind: manual_label`, prompt for a photo |
