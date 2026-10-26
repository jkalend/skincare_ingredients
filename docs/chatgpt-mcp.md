# Connect ChatGPT to the skincare MCP server

The Go binary can serve a **read-only MCP endpoint**. ChatGPT can connect to it
over HTTPS. It exposes public data only: no shelf, no scraped panels, no writes.

## What ChatGPT gets

| Tool | What it does |
|---|---|
| `lookup_product` | barcode → cached product, then Open Beauty Facts |
| `search_products` | brand/name/barcode substring search |
| `parse_ingredients` | label text → ordered token list |
| `analyze_inci` | label text → canonical names, functions, annex citations |
| `get_ingredient` | one canonical ingredient + annex matches |
| `doctor` | registry version and data counts |

Every tool description carries the caveat: ingredient order is not
concentration; below 1% any order is legal; annex matches are citations, not
safety or compliance verdicts.

## Run it locally

```bash
skincare mcp --transport http --addr 127.0.0.1:8080 --read-only
```

Health check:

```bash
curl http://127.0.0.1:8080/healthz
```

MCP endpoint: `http://127.0.0.1:8080/mcp`

On Windows, if `skincare` is not on your PATH:

```powershell
go run ./cmd/skincare mcp --transport http --addr 127.0.0.1:8080 --read-only
```

## Give ChatGPT a public HTTPS URL (free, no card)

ChatGPT connects to **remote** MCP servers. While developing, a free Cloudflare
quick tunnel is enough:

1. Install `cloudflared` (free, no account required):
   `winget install --id Cloudflare.cloudflared`
2. Start the MCP server:
   ```bash
   skincare mcp --transport http --addr 127.0.0.1:8080 --read-only
   ```
3. In a second terminal, expose it:
   ```bash
   cloudflared tunnel --url http://127.0.0.1:8080
   ```
4. Copy the printed `https://<random>.trycloudflare.com` URL and append `/mcp`.
5. In ChatGPT: **Settings → Connectors / Apps → Add custom MCP server**, use the
   `https://…trycloudflare.com/mcp` URL. The endpoint is read-only.
6. Ask:
   ```text
   Use the skincare MCP server. Look up barcode 1000034026006, list the
   ingredients with canonical names and any annex citations, and tell me which
   tokens were unresolved. Do not infer concentrations from order.
   ```

The quick tunnel is public while it runs. The server is read-only and hides the
shelf, but stop the tunnel when you are done. If you want a shared secret, start
the server with:

```bash
skincare mcp --transport http --addr 127.0.0.1:8080 --read-only --token "long-random-string"
```

Then configure the ChatGPT connector with API-key auth:
header `Authorization`, value `Bearer long-random-string`.

## What it does not expose

- `shelf_list`, `shelf_add`, `shelf_rm` are omitted in `--read-only` mode.
- `local/` (private shelf, scraped panels) is not loaded in read-only mode.
- No file writes except the public OBF product cache in `data/products/`.
- No OCR, no scraping of arbitrary URLs, no model calls.

## Plan notes

MCP support in ChatGPT depends on the plan. At the time of writing, Pro can
connect custom MCP servers with read/fetch permissions in developer mode;
full MCP and workspace publishing are Business/Enterprise/Edu. The server here
is read-only, so the read/fetch path is the intended one. If your plan cannot
add a custom connector, use the CLI or the local stdio MCP server instead.

## Local agent (stdio)

For Codex/Claude Code/another local agent, no tunnel is needed:

```bash
skincare mcp --transport stdio
```

The stdio server includes the shelf tools unless `--read-only` is passed. It
speaks MCP on stdin/stdout and logs on stderr.
