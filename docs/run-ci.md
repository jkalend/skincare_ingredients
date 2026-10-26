# Running CI and having ChatGPT read the result

Plain version: **ChatGPT cannot press a GitHub button by itself.** It can read
a public repo, and it can call a tool you give it. To make ChatGPT *start* CI,
give it a small GitHub Action/MCP tool with a token. If you don't want to set
that up, start the run yourself and ask ChatGPT to read the result file.

This repo runs CI **only when invoked**. The workflow has
`on: workflow_dispatch` — no schedule, no push trigger, no pull-request trigger.
Every run writes `ci/last-run.json` on `main`, so the answer is a public file,
not a log hunt.

## What a run produces

```json
{
  "schema_version": 1,
  "status": "completed",
  "conclusion": "success",
  "workflow": "ci",
  "event": "workflow_dispatch",
  "run_id": "1234567890",
  "run_url": "https://github.com/jkalend/skincare_ingredients/actions/runs/1234567890",
  "commit": "abc123…",
  "ref": "main",
  "reason": "manual",
  "checks": { "docs_links": "success" },
  "details": ["OK: 12 Markdown files; required files present; links resolve"],
  "generated_at": "2026-10-26T15:00:00Z"
}
```

The workflow currently checks docs links. When the Go code lands, the same job
gets `go test ./...` and the parser corpus gate; the result shape stays.

## Option A — you start it, ChatGPT reads it (simplest, no token in ChatGPT)

Run one of these from your machine:

```bash
gh workflow run ci.yml --repo jkalend/skincare_ingredients --ref main -f reason="manual check"
gh run list --repo jkalend/skincare_ingredients --workflow ci.yml --limit 1
gh run watch <RUN_ID> --repo jkalend/skincare_ingredients --exit-status
```

Then give ChatGPT this prompt:

```text
Read https://raw.githubusercontent.com/jkalend/skincare_ingredients/main/ci/last-run.json
and summarize:
1. conclusion
2. commit, run URL, and reason
3. which check failed, and the exact error lines
4. if it succeeded, any counts or hashes in the details
Do not claim success unless conclusion == "success".
```

That works with any ChatGPT plan and no GitHub token. The repository is public;
the raw file can be read without authentication. If ChatGPT cannot browse,
paste the JSON into the chat.

You can also run it from the GitHub UI: **Actions → ci → Run workflow → main**.

## Option B — ChatGPT starts the run (Custom GPT Action or MCP)

You need three things:

1. The public repo and the `ci` workflow on `main`.
2. A **fine-grained GitHub token** for that one repo:
   - Actions: **Read and write** (dispatch + status)
   - Contents: **Read** (read `ci/last-run.json`; write is only needed by the
     workflow itself, which uses `GITHUB_TOKEN`)
   - Metadata: Read (automatic)
   - Short expiry; rotate it.
3. A Custom GPT Action (or an MCP server) that calls the GitHub REST API. Put
   the token in the Action's API-key auth or in a tiny proxy secret — **never
   paste it into the chat**.

The Action needs four calls:

| Purpose | Call |
|---|---|
| Start | `POST /repos/{owner}/{repo}/actions/workflows/ci.yml/dispatches` with `{"ref":"main"}` |
| Find the run | `GET /repos/{owner}/{repo}/actions/runs?event=workflow_dispatch&per_page=1` |
| Check status | `GET /repos/{owner}/{repo}/actions/runs/{run_id}` |
| Read result | `GET /repos/{owner}/{repo}/contents/ci/last-run.json?ref=main` |

GitHub sends `204 No Content` for the dispatch, without a run id. That is why
the Action must poll the runs list. Use `Accept: application/vnd.github+json`
and `X-GitHub-Api-Version: 2022-11-28` headers.

Then this prompt works:

```text
Use the GitHub CI action to start the "ci" workflow in
jkalend/skincare_ingredients on main, with reason "manual check".
Poll the latest workflow_dispatch run until status is "completed".
Then read ci/last-run.json and report:
1. conclusion
2. commit, run URL, and reason
3. which check failed, with the exact error lines
4. any counts, hashes, or freshness warnings in the details
Never call it a success unless conclusion == "success".
If the run is still queued after 2 minutes, say so and give me the run URL.
```

Why a proxy is safer: the token stays in the proxy/Action auth, not in the
conversation. A Cloudflare Worker on the free plan (no card) can expose
`POST /run` and `GET /status`, hold the fine-grained token, and return only the
status fields. If you use the Action directly against GitHub, the token is
still scoped to one repo and Actions; the risk is smaller but not zero.

## What ChatGPT can and cannot do

| It can | It cannot |
|---|---|
| Read the public repo and `ci/last-run.json` | Press “Run workflow” without a tool/token |
| Call a Custom GPT Action or MCP tool you configured | Use the built-in GitHub connector as a CI trigger |
| Read run status, jobs and public logs via an Action | Write to the repo with a read-only token |
| Summarize the result file in plain language | Run a local `gh` command on your machine |

If ChatGPT says “I can't run workflows”, it is not broken — it has no trigger
tool. Use Option A, or configure Option B.

## If the run fails

1. Open the `run_url` from `ci/last-run.json`.
2. The failing step is in `checks` / `details`; the workflow also fails the job
   after writing the file, so the red X and the JSON agree.
3. If `ci/last-run.json` is missing or old, the commit step failed. Check the
   run logs for `git push`; branch protection or a missing `contents: write`
   permission is the usual cause.
4. A `workflow_dispatch` call can return 404 if the token cannot see the repo,
   the workflow file is not on the default branch, or `Actions: write` is
   missing.

## Free?

- Public repo + standard GitHub-hosted runner: free, no minute cap.
- Fine-grained PAT: free.
- Custom GPT Action / MCP: depends on your ChatGPT plan; the built-in connector
  is not a trigger.
- Optional Cloudflare Worker proxy: free plan, no card.
- No nightly job, so no idle runs and no 60-day schedule auto-disable.
