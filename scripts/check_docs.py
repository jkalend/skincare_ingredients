#!/usr/bin/env python3
"""Check local Markdown links and required files.

No third-party dependencies; runs on the GitHub Actions ubuntu-latest image.
Exit 1 on a broken relative link or a missing required file.
"""

from __future__ import annotations

import re
import sys
import urllib.parse
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

SKIP_DIRS = {".git", ".tmp", "cache", "local", "node_modules", "venv", ".venv"}
REQUIRED = [
    "README.md",
    "AGENTS.md",
    "DATA-LICENSE.md",
    "docs/research.md",
    "docs/review-2026-10-26.md",
    "docs/scraping.md",
    "docs/agent-interface.md",
    "docs/run-ci.md",
    "docs/decisions/0001-language.md",
    "docs/decisions/0002-data-and-ci.md",
]

FENCE = re.compile(r"^\s*(```|~~~)")
INLINE_CODE = re.compile(r"`[^`\n]*`")
LINK = re.compile(r"!?\[[^\]]*\]\(([^)\s]+)(?:\s+['\"][^'\"]*['\"])?\)")


def markdown_files() -> list[Path]:
    files: list[Path] = []
    for path in ROOT.rglob("*.md"):
        if any(part in SKIP_DIRS for part in path.relative_to(ROOT).parts):
            continue
        files.append(path)
    return sorted(files)


def strip_code(text: str) -> str:
    out: list[str] = []
    in_fence = False
    for line in text.splitlines():
        if FENCE.match(line):
            in_fence = not in_fence
            out.append("")
            continue
        out.append("" if in_fence else INLINE_CODE.sub("", line))
    return "\n".join(out)


def check_link(source: Path, raw: str) -> str | None:
    target = raw.strip().strip("<>")
    if not target or target.startswith("#"):
        return None
    if target.startswith(("http://", "https://", "mailto:", "tel:", "data:")):
        return None
    target = urllib.parse.unquote(target.split("#", 1)[0].split("?", 1)[0])
    if not target:
        return None
    resolved = (source.parent / target).resolve()
    try:
        resolved.relative_to(ROOT)
    except ValueError:
        return f"{source.relative_to(ROOT)}: link escapes repo: {raw}"
    if not resolved.exists():
        return f"{source.relative_to(ROOT)}: missing {raw}"
    return None


def main() -> int:
    problems: list[str] = []

    for required in REQUIRED:
        if not (ROOT / required).is_file():
            problems.append(f"missing required file: {required}")

    files = markdown_files()
    for path in files:
        text = strip_code(path.read_text(encoding="utf-8", errors="replace"))
        for match in LINK.finditer(text):
            problem = check_link(path, match.group(1))
            if problem:
                problems.append(problem)

    if problems:
        print(f"FAIL: {len(problems)} problem(s) in {len(files)} Markdown files")
        for problem in problems:
            print(f"  - {problem}")
        return 1

    print(f"OK: {len(files)} Markdown files; required files present; links resolve")
    return 0


if __name__ == "__main__":
    sys.exit(main())
