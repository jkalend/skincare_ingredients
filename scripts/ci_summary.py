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

MAX_DETAIL_LINES = 30
MAX_TOTAL_DETAIL_LINES = 80


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
    parser.add_argument("--go-outcome", default="skipped")
    parser.add_argument("--build-outcome", default="skipped")
    parser.add_argument("--reason", default="manual")
    parser.add_argument("--details-file")
    parser.add_argument("--go-details-file")
    parser.add_argument("--build-details-file")
    parser.add_argument("--binary")
    parser.add_argument("--output", default="ci/last-run.json")
    args = parser.parse_args()

    run_id = env("GITHUB_RUN_ID")
    server = env("GITHUB_SERVER_URL", "https://github.com").rstrip("/")
    repo = env("GITHUB_REPOSITORY")
    run_url = f"{server}/{repo}/actions/runs/{run_id}" if run_id and repo else ""

    checks = {
        "docs_links": args.docs_outcome,
        "go_test": args.go_outcome,
        "go_build": args.build_outcome,
    }
    ok = all(value in ("success", "skipped") for value in checks.values())

    details: list[str] = []
    for path in (args.go_details_file, args.build_details_file, args.details_file):
        if path:
            details.extend(tail(path))
    details = details[-MAX_TOTAL_DETAIL_LINES:]

    binary_bytes = None
    if args.binary:
        binary = Path(args.binary)
        if binary.is_file():
            binary_bytes = binary.stat().st_size

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
        "checks": checks,
        "binary_bytes": binary_bytes,
        "details": details,
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    }

    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"wrote {output}: {summary['conclusion']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
