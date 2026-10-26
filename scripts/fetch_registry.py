#!/usr/bin/env python3
"""Fetch the free CosIng registry snapshot used by the Go tool.

Sources:
  * official annex CSVs: api.tech.ec.europa.eu/cosing20/1.0/api/annexes/{II..VI}/export-csv
  * CosIng Checker mirror: functions + glossary + ingredient records (CC BY 4.0)

The mirror has no bulk ingredient endpoint, so this walks the 83 functions and
merges their ingredient records with the EU 2025/1175 glossary. It is slow by
design: 120 requests/minute, one request every 0.55 s, no parallel calls.

Run from the repo root:
    python scripts/fetch_registry.py
"""

from __future__ import annotations

import csv
import hashlib
import io
import json
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "data" / "registry" / "cosing"
MIRROR = "https://cosingchecker.com"
API = MIRROR + "/api/v1"
UA = "SkincareIngredients/0.1 (+https://github.com/jkalend/skincare_ingredients; personal research)"
SLEEP = 0.55
RETRIES = 4
ANNEXES = ("II", "III", "IV", "V", "VI")


def now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def get(url: str) -> bytes:
    last: Exception | None = None
    for attempt in range(RETRIES):
        req = urllib.request.Request(url, headers={"User-Agent": UA})
        try:
            with urllib.request.urlopen(req, timeout=60) as response:
                body = response.read()
            time.sleep(SLEEP)
            return body
        except urllib.error.HTTPError as exc:
            last = exc
            if exc.code in (429, 500, 502, 503, 504):
                time.sleep(2 ** attempt)
                continue
            raise
        except Exception as exc:  # network reset, timeout, ...
            last = exc
            time.sleep(2 ** attempt)
    raise RuntimeError(f"GET {url} failed: {last}")


def get_json(path: str, params: dict[str, object] | None = None) -> dict:
    url = API + path
    if params:
        url += "?" + urllib.parse.urlencode(params)
    return json.loads(get(url))


def fetch_pages(path: str, params: dict[str, object] | None = None) -> list[dict]:
    page = 1
    out: list[dict] = []
    while True:
        query = dict(params or {})
        query.update({"page": page, "per_page": 100})
        data = get_json(path, query)
        results = data.get("results", [])
        out.extend(results)
        if not data.get("next") or not results:
            return out
        page += 1


def split_ref_rows(text: str) -> tuple[list[list[str]], int, str, str]:
    rows = list(csv.reader(io.StringIO(text)))
    header_row = next(
        (i for i, row in enumerate(rows) if row and row[0] == "Reference Number"),
        None,
    )
    if header_row is None:
        raise ValueError("annex has no Reference Number header")
    data_rows = [
        row
        for row in rows[header_row + 1 :]
        if row and re.match(r"^\d+(?:\s*[A-Za-z])?$", row[0].strip())
    ]
    creation = rows[0][0].partition(":")[2].strip() if rows and rows[0] else ""
    update = rows[1][1].partition(":")[2].strip() if len(rows) > 1 and len(rows[1]) > 1 else ""
    return data_rows, len(data_rows), creation, update


def fetch_annexes(manifest: dict) -> None:
    annex_manifest: dict[str, dict] = {}
    for annex in ANNEXES:
        url = f"https://api.tech.ec.europa.eu/cosing20/1.0/api/annexes/{annex}/export-csv"
        raw = get(url)
        path = OUT / f"annex_{annex}.csv"
        path.write_bytes(raw)

        text = raw.decode("utf-8-sig")
        rows, count, creation, update = split_ref_rows(text)
        lines = raw.splitlines(keepends=True)
        content = b"".join(lines[2:])
        annex_manifest[annex] = {
            "source_url": url,
            "fetched_at": now(),
            "raw_sha256": hashlib.sha256(raw).hexdigest(),
            "raw_bytes": len(raw),
            "content_sha256": hashlib.sha256(content).hexdigest(),
            "file_creation_date": creation,
            "last_update": update,
            "data_rows": count,
        }
        print(f"annex {annex}: {count} rows, last update {update}", flush=True)

    manifest["annexes"] = annex_manifest


def clean(value: object) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None


def merge_ingredient(
    by_name: dict[str, dict],
    record: dict,
    *,
    function: str | None,
    fetched_at: str,
    source_kind: str,
) -> None:
    name = clean(record.get("inci_name"))
    if not name:
        return
    key = name.casefold()
    current = by_name.get(key)
    if current is None:
        current = {
            "inci_name": name,
            "inn_name": clean(record.get("inn_name")),
            "ph_eur_name": clean(record.get("ph_eur_name")),
            "cas_number": clean(record.get("cas_number")),
            "ec_number": clean(record.get("ec_number")),
            "description": clean(record.get("description")),
            "restriction": clean(record.get("restriction")),
            "ref_number": clean(record.get("ref_number")),
            "functions": [],
            "glossary_entry": None,
            "source": {
                "source": "cosingchecker",
                "url": MIRROR + record.get("url", "/"),
                "fetched_at": fetched_at,
                "method": "mirror",
                "confidence": 0.9 if source_kind == "ingredient" else 0.8,
            },
        }
        by_name[key] = current

    for field in ("inn_name", "ph_eur_name", "cas_number", "ec_number", "description", "restriction", "ref_number"):
        if not current.get(field) and clean(record.get(field)):
            current[field] = clean(record.get(field))

    functions = record.get("functions")
    if not functions and function:
        functions = [function]
    for item in functions or []:
        item = str(item).strip().upper()
        if item and item not in current["functions"]:
            current["functions"].append(item)
    current["functions"].sort()


def fetch_mirror() -> tuple[list[dict], dict]:
    fetched_at = now()

    print("functions...", flush=True)
    functions = fetch_pages("/functions/")
    function_names = sorted({str(item["name"]).strip().upper() for item in functions if item.get("name")})
    print(f"  {len(function_names)} functions", flush=True)

    by_name: dict[str, dict] = {}
    for index, function in enumerate(function_names, 1):
        slug = str(function).lower().replace(" ", "-")
        # The API's slug is not always the name lowercased with dashes; use the
        # record's own slug from the list where possible.
        record = next((item for item in functions if str(item.get("name", "")).strip().upper() == function), None)
        if record and record.get("slug"):
            slug = str(record["slug"])
        try:
            records = fetch_pages(f"/functions/{slug}/ingredients/")
        except Exception as exc:
            print(f"  ! {function}: {exc}", flush=True)
            continue
        for item in records:
            merge_ingredient(by_name, item, function=function, fetched_at=fetched_at, source_kind="ingredient")
        if index % 10 == 0 or index == len(function_names):
            print(f"  {index}/{len(function_names)} functions, {len(by_name)} ingredients", flush=True)

    print("glossary...", flush=True)
    glossary = fetch_pages("/inci-glossary/")
    glossary_version = ""
    for item in glossary:
        name = clean(item.get("common_name"))
        if not name:
            continue
        key = name.casefold()
        current = by_name.get(key)
        if current is None:
            current = {
                "inci_name": name,
                "inn_name": None,
                "ph_eur_name": None,
                "cas_number": None,
                "ec_number": None,
                "description": None,
                "restriction": None,
                "ref_number": None,
                "functions": [],
                "glossary_entry": item.get("entry_number"),
                "source": {
                    "source": "cosingchecker",
                    "url": clean(item.get("source_url")) or MIRROR + "/",
                    "fetched_at": fetched_at,
                    "method": "mirror",
                    "confidence": 0.8,
                },
            }
            by_name[key] = current
        elif not current.get("glossary_entry"):
            current["glossary_entry"] = item.get("entry_number")
        glossary_version = clean(item.get("source_version")) or glossary_version
    print(f"  {len(glossary)} glossary rows, {len(by_name)} merged names", flush=True)

    return sorted(by_name.values(), key=lambda x: x["inci_name"]), {
        "source": "cosingchecker",
        "url": API,
        "fetched_at": fetched_at,
        "function_count": len(function_names),
        "glossary_count": len(glossary),
        "ingredient_count": len(by_name),
        "glossary_version": glossary_version,
        "license": "CC BY 4.0 (site declaration); source data: European Commission CosIng",
    }


def main() -> int:
    OUT.mkdir(parents=True, exist_ok=True)
    manifest: dict = {
        "schema_version": 1,
        "generated_by": "scripts/fetch_registry.py",
        "fetched_at": now(),
    }
    fetch_annexes(manifest)
    ingredients, mirror_manifest = fetch_mirror()
    manifest["mirror"] = mirror_manifest

    (OUT / "ingredients.json").write_text(
        json.dumps(ingredients, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n",
        encoding="utf-8",
    )
    (OUT / "manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    print(f"wrote {len(ingredients)} ingredient records to {OUT}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
