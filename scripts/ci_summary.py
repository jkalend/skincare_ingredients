#!/usr/bin/env python3
"""Write ci/last-run.json from a GitHub Actions run.

The file is deliberately small and stable: ChatGPT or an agent reads it after
`workflow_dispatch` instead of parsing the Actions UI.
"""

from __future__ import annotations

import argparse
import json
import os
from datetime import datetime, timezone
from pathlib import Path

MAX_DETAIL_LINES = 40


def env(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


def tail(path: str | None) -> list[str]:
    if not path:
        return []
    p = Path(path)
    if not p.is_file():
        return []
    lines = p.read_text(encoding="utf-8", errors="replace").splitlines()
    return lines[-MAX_DETAIL_LINES:]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--docs-outcome", required=True)
    parser.add_argument("--reason", default="manual")
    parser.add_argument("--details-file")
    parser.add_argument("--output", default="ci/last-run.json")
    args = parser.parse_args()

    run_id = env("GITHUB_RUN_ID")
    server = env("GITHUB_SERVER_URL", "https://github.com").rstrip("/")
    repo = env("GITHUB_REPOSITORY")
    run_url = f"{server}/{repo}/actions/runs/{run_id}" if run_id and repo else ""

    ok = args.docs_outcome == "success"
    summary = {
        "schema_version": 1,
        "status": "completed",
        "conclusion": "success" if ok else "failure",
        "workflow": env("GITHUB_WORKFLOW", "ci"),
        "event": env("GITHUB_EVENT_NAME", "workflow_dispatch"),
        "run_id": run_id,
        "run_attempt": env("GITHUB_RUN_ATTEMPT"),
        "run_url": run_url,
        "commit": env("GITHUB_SHA"),
        "ref": env("GITHUB_REF_NAME"),
        "reason": args.reason,
        "checks": {"docs_links": args.docs_outcome},
        "details": tail(args.details_file),
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    }

    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"wrote {output}: {summary['conclusion']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
