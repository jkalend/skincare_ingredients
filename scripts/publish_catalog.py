#!/usr/bin/env python3
"""Publish an owner-authorized, privacy-filtered product/formula catalog.

Reads the private inventory export, removes shelf and ownership metadata, and
writes deterministic JSON and Markdown views under data/catalog/.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parent.parent
INPUT = ROOT / "local" / "exports" / "owned-inventory.json"
OUTPUT_DIR = ROOT / "data" / "catalog"
JSON_OUTPUT = OUTPUT_DIR / "products.json"
MARKDOWN_OUTPUT = OUTPUT_DIR / "products.md"

SOURCE_OVERRIDES = {
    "cosrx-snail-92-cream": {
        "source": "retailer",
        "url": "https://www.notino.cz/cosrx/advanced-snail-92-all-in-one-intenzivni-regeneracni-krem-se-snecim-extraktem/p-16121751/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.95,
    },
    "cosrx-snail-essence-96": {
        "source": "retailer",
        "url": "https://www.notino.cz/cosrx/advanced-snail-96-mucin-pletova-esence-se-snecim-extraktem/p-15811222/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.95,
    },
    "cosrx-snail-peptide-eye-cream": {
        "source": "retailer",
        "url": "https://www.notino.cz/cosrx/advanced-snail-peptide-eye-cream-rozjasnujici-a-hydratacni-krem-na-ocni-okoli/p-16307300/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.95,
    },
    "cosrx-snail-radiance-essence": {
        "source": "retailer",
        "url": "https://www.notino.cz/cosrx/advanced-snail-radiance-dual-essence-rozjasnujici-hydratacni-serum-s-hlemyzdim-extraktem/p-16313428/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.95,
    },
    "ordinary-pha-lip-serum": {
        "source": "retailer",
        "url": "https://www.notino.cz/the-ordinary/pha-5-exfoliating-lip-serum-vyhlazujici-exfoliacni-serum-na-rty/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.95,
    },
    "ordinary-salicylic-anhydrous": {
        "source": "retailer",
        "url": "https://www.notino.cz/the-ordinary/salicylic-acid-2-anhydrous-solution-pletove-serum-pro-jednotny-ton-pleti/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.95,
    },
    "nivea-luminous630-antipigment-serum": {
        "source": "retailer",
        "url": "https://www.notino.cz/nivea/cellular-luminous-630-intenzivni-serum-proti-pigmentovym-skvrnam/p-16063704/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.9,
    },
    "skin1004-poremizing-cream": {
        "source": "retailer",
        "url": "https://www.sephora.co.uk/p/skin1004-madagascar-centella-poremizing-light-gel-cream",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.95,
    },
    "vt-reedle-shot-700": {
        "source": "retailer",
        "url": "https://www.dokdo.cz/vt-cosmetics-mikrojehlickovy-booster-reedle-shot-700/",
        "fetched_at": None,
        "method": "retailer_page",
        "confidence": 0.9,
    },
}

# Notino's listed sequence was checked for these four formulas. This also
# corrects an imported comma split in the 96 Mucin formula.
FORMULA_OVERRIDES = {
    "cosrx-snail-92-cream": [
        "Snail Secretion Filtrate", "Betaine", "Caprylic/Capric Triglyceride",
        "Cetearyl Olivate", "Sorbitan Olivate", "Cetearyl Alcohol", "Carbomer",
        "Arginine", "Dimethicone", "Sodium Polyacrylate", "Phenoxyethanol",
        "Sodium Hyaluronate", "Stearic Acid", "Allantoin", "Panthenol",
        "Xanthan Gum", "Ethyl Hexanediol", "Adenosine",
    ],
    "cosrx-snail-essence-96": [
        "Snail Secretion Filtrate", "Betaine", "Butylene Glycol",
        "1,2-Hexanediol", "Sodium Hyaluronate", "Panthenol", "Arginine",
        "Allantoin", "Ethyl Hexanediol", "Sodium Polyacrylate", "Carbomer",
        "Phenoxyethanol",
    ],
    "cosrx-snail-peptide-eye-cream": [
        "Snail Secretion Filtrate", "Butylene Glycol", "Glycerin",
        "Helianthus Annuus (Sunflower) Seed Oil", "1,2-Hexanediol",
        "Niacinamide", "Water", "Palmitic Acid", "Arginine", "Carbomer",
        "Stearic Acid", "Betaine", "Cetearyl Olivate", "Sorbitan Olivate",
        "Sodium Polyacrylate", "Allantoin", "Tocopheryl Acetate", "Panthenol",
        "Hydroxyethyl Acrylate/Sodium Acryloyldimethyl Taurate Copolymer",
        "Sodium Hyaluronate", "Adenosine", "Xanthan Gum", "Myristic Acid",
        "Copper Tripeptide-1", "Acetyl Hexapeptide-8", "Palmitoyl Pentapeptide-4",
        "Palmitoyl Tripeptide-1", "Palmitoyl Tetrapeptide-7",
    ],
    "cosrx-snail-radiance-essence": [
        "Snail Secretion Filtrate", "Butylene Glycol", "Niacinamide",
        "1,2-Hexanediol", "Betaine", "Panthenol", "Water", "Glycerin",
        "Limnanthes Alba (Meadowfoam) Seed Oil",
        "Helianthus Annuus (Sunflower) Seed Oil", "Allantoin", "Carbomer",
        "Macadamia Ternifolia Seed Oil", "Sodium Polyacrylate",
        "Sodium Hyaluronate", "Inulin Lauryl Carbamate", "Arginine",
        "Argania Spinosa Kernel Oil", "Xanthan Gum", "Sodium Surfactin",
        "Ethylhexylglycerin", "Glutathione",
    ],
    "nivea-luminous630-antipigment-serum": [
        "Aqua", "Isopropyl Palmitate", "Cetearyl Isononanoate", "Dimethicone",
        "Glycerin", "Alcohol Denat.", "Cocoglycerides", "Distarch Phosphate",
        "Butylene Glycol", "Methylpropanediol", "Tapioca Starch",
        "Isobutylamido Thiazolyl Resorcinol", "Sodium Hyaluronate",
        "Tocopheryl Acetate", "Stearyl Alcohol", "Cetyl Alcohol",
        "Glyceryl Stearate", "Sodium Ascorbyl Phosphate",
        "Sodium Stearoyl Glutamate", "Sodium Polyacrylate",
        "Acrylates/C10-30 Alkyl Acrylate Crosspolymer", "Trisodium EDTA",
        "Sodium Chloride", "Sodium Hydroxide", "Sodium Sulfate",
        "Phenoxyethanol", "Benzyl Alcohol", "Parfum", "CI 15985", "CI 47005",
    ],
    "vt-reedle-shot-700": [
        "Aqua (Water)", "Glycerin", "Methylpropanediol", "Propanediol",
        "Caprylic/Capric Triglyceride", "Butylene Glycol", "Cetyl Ethylhexanoate",
        "Glycereth-26", "Pentaerythrityl Tetraethylhexanoate",
        "Butyrospermum Parkii (Shea) Butter", "Macadamia Ternifolia Seed Oil",
        "Niacinamide", "Silica", "Polyglyceryl-3 Methylglucose Distearate",
        "Cetearyl Alcohol", "Stearic Acid", "1,2-Hexanediol", "C12-16 Alcohols",
        "Sodium Acrylate/Sodium Acryloyldimethyl Taurate Copolymer", "Polyisobutene",
        "Hydrogenated Lecithin", "Palmitic Acid", "Arginine", "Cetearyl Glucoside",
        "Carbomer", "Capryl Glycol", "Ethylhexylglycerin", "Centella Asiatica Extract",
        "Adenosine", "Caprylyl/Capryl Glucoside", "Sorbitan Oleate", "Disodium EDTA",
        "Tocopheryl Acetate", "Sodium Hyaluronate",
        "Acrylates/C10-30 Alkyl Acrylate Crosspolymer", "Glucose", "Propolis Extract",
        "Asiaticoside", "Hydrolyzed Elastin", "Asiatic Acid", "Madecassic Acid",
        "Madecassoside", "Glycine", "Serine", "Glutamic Acid", "Aspartic Acid",
        "Leucine", "Alanine", "Lysine", "Tyrosine", "Phenylalanine", "Threonine",
        "Valine", "Isoleucine", "Ceramide NP", "Histidine", "Cysteine", "Methionine",
        "Glycosphingolipids",
    ],
}

SUPPORTING_SOURCES_BY_PRODUCT = {
    "boj-relief-sun-aqua-fresh": [
        {
            "source": "Beauty of Joseon",
            "url": "https://beautyofjoseon.com/products/relief-sun-aqua-fresh",
            "fetched_at": None,
            "method": "manufacturer_name_cross_check",
            "confidence": 0.99,
        },
    ],
    "dr-althea-147-barrier": [
        {
            "source": "Dr. Althea",
            "url": "https://doctoraltheaglobal.com/products/dralthea-147_barrier_cream",
            "fetched_at": None,
            "method": "manufacturer_name_cross_check",
            "confidence": 0.95,
        },
    ],
    "ziaja-goat-milk-day": [
        {
            "source": "Ziaja Czech Republic",
            "url": "https://cz.ziaja.com/product/10807-kozi-mleko-denni-krem",
            "fetched_at": None,
            "method": "manufacturer_formula_cross_check",
            "confidence": 0.98,
        },
    ],
    "ziaja-goat-milk-night": [
        {
            "source": "Ziaja Czech Republic",
            "url": "https://cz.ziaja.com/product/10811-kozi-mleko-nocni-krem",
            "fetched_at": None,
            "method": "manufacturer_formula_cross_check",
            "confidence": 0.98,
        },
    ],
    "nivea-luminous630-antipigment-serum": [
        {
            "source": "Dr. Max Lékárna",
            "url": "https://www.drmax.cz/nivea-luminous-serum-proti-pigmentovym-skvrnam-30-ml",
            "fetched_at": None,
            "method": "retailer_cross_check",
            "confidence": 0.9,
        },
    ],
}

NORMALIZATION_NOTES = {
    "medipeel-peptide9-eye-patches": [
        "The name 'Peptide 9 eye patches' does not identify one exact formula; "
        "the specific variant is unknown, so no INCI list is assigned."
    ],
    "dr-althea-147-barrier": [
        "The retailer prints 'Sodium Hyaluronate Dimethiconol'; the maker's "
        "current product page lists 'Sodium Hyaluronate Dimethylsilanol'. "
        "The canonical name follows the maker; check the package for the "
        "specific production version."
    ],
    "nivea-luminous630-antipigment-serum": [
        "Notino and Dr. Max list this ordered formula for the product. "
        "Formulas can change by production run; compare with the package to "
        "confirm the version in hand."
    ],
    "vt-reedle-shot-700": [
        "The ordered formula is transcribed from the linked Czech retailer; "
        "check the package if confirming a specific production version."
    ],
}

PUBLIC_ID_OVERRIDES = {"pleva-aspintoxin-cream": "pleva-bee-venom-cream"}

PLEVA_SUPPORTING_SOURCES = [
    {
        "source": "Hallstar BPC",
        "url": "https://www.hallstarbeauty.com/product/olivem-1000/",
        "fetched_at": None,
        "method": "supplier_inci_reference",
        "confidence": 0.98,
    },
    {
        "source": "SOPHIM",
        "url": "https://www.sophim.com/en/product/biophytosebum/",
        "fetched_at": None,
        "method": "supplier_inci_reference",
        "confidence": 0.98,
    },
    {
        "source": "Hallstar BPC",
        "url": "https://www.hallstarbeauty.com/product/oliwax/",
        "fetched_at": None,
        "method": "supplier_inci_reference",
        "confidence": 0.98,
    },
    {
        "source": "Hallstar BPC",
        "url": "https://www.hallstarbeauty.com/product/olivem-vs-feel/",
        "fetched_at": None,
        "method": "supplier_inci_reference",
        "confidence": 0.98,
    },
]

PLEVA_NOTE = (
    "Pleva presents several multi-INCI blends as adjacent names without "
    "separators; those names are listed as separate ordered ingredients. "
    "The page's 'Olivem VS Fell' text is treated as a repeated trade-name "
    "reference: Hallstar identifies Olivem VS Feel as the same four INCI "
    "components immediately preceding it, so no duplicate ingredients are "
    "added. The source spells 'Hdrogenated' and 'Ethylhexylglycerine'; the "
    "source spellings are retained while canonical names are corrected."
)


def read_export() -> list[dict[str, Any]]:
    if not INPUT.is_file():
        raise ValueError(
            f"Private export not found at {INPUT}; create it with the local "
            "inventory export before publishing."
        )
    data = json.loads(INPUT.read_text(encoding="utf-8"))
    if not isinstance(data, dict) or not isinstance(data.get("products"), list):
        raise ValueError("Private export must contain a products array.")
    rows = data["products"]
    if not rows:
        raise ValueError("Private export contains no products.")
    if any(row.get("shelf") != "owned" for row in rows):
        raise ValueError("Only the explicit owned-inventory export can be published.")
    return rows


def source_record(source: dict[str, Any] | None) -> dict[str, Any] | None:
    if not source:
        return None
    record = {
        key: source.get(key)
        for key in ("source", "url", "fetched_at", "method", "confidence")
    }
    if not all(record.get(key) is not None for key in ("source", "url", "method", "confidence")):
        raise ValueError(f"Incomplete source provenance: {record}")
    if not record["url"].startswith("https://"):
        raise ValueError(f"Expected an HTTPS source URL: {record['url']}")
    return record


def ingredient_from_export(
    item: dict[str, Any], unresolved_by_position: dict[int, dict[str, Any]]
) -> dict[str, Any]:
    position = int(item["position"])
    printed = str(item.get("printed", ""))
    canonical = item.get("canonical")
    unresolved = unresolved_by_position.get(position)
    aliases = {
        "aqua/water/eau": ("AQUA", "synonym_map", 0.98),
        "aqua/water": ("AQUA", "synonym_map", 0.98),
        "water/aqua": ("AQUA", "synonym_map", 0.98),
        "parfum/fragrance": ("PARFUM", "synonym_map", 0.98),
        "terephthalidene dicamphor sulfonic acid": (
            "TEREPHTHALYLIDENE DICAMPHOR SULFONIC ACID",
            "source_spelling_correction",
            0.99,
        ),
        "sodium hyaluronate dimethiconol": (
            "SODIUM HYALURONATE DIMETHYLSILANOL",
            "manufacturer_name_cross_check",
            0.95,
        ),
        "3-0-ethyl ascorbic acid": (
            "3-O-ETHYL ASCORBIC ACID",
            "registry_normalization",
            0.99,
        ),
        "helianthus annuus seed cera (helianthus annuus (sunflower) seed wax)": (
            "HELIANTHUS ANNUUS SEED CERA",
            "registry_normalization",
            0.99,
        ),
        "lactobacillus/milk solids/glycine soja (soybean) oil ferment": (
            "LACTOBACILLUS/MILK SOLIDS/GLYCINE SOJA (SOYBEAN) OIL FERMENT",
            "manufacturer_formula_cross_check",
            0.98,
        ),
    }
    alias = aliases.get(printed.casefold())
    if alias:
        return {
            "position": position,
            "printed": printed,
            "canonical": alias[0],
            "resolution": "resolved",
            "method": alias[1],
            "confidence": alias[2],
        }
    if canonical is None or item.get("method") == "unresolved" or unresolved:
        return {
            "position": position,
            "printed": printed,
            "canonical": None,
            "resolution": "unresolved",
            "method": "unresolved",
            "confidence": 0,
            "reason": (unresolved or {}).get("reason", "not resolved in the source import"),
        }
    return {
        "position": position,
        "printed": printed,
        "canonical": canonical,
        "resolution": "resolved",
        "method": item.get("method") or "unknown",
        "confidence": item.get("confidence"),
    }


def resolve_positions(product: dict[str, Any]) -> list[dict[str, Any]]:
    key = str(product["product_key"]).removeprefix("local:")
    if key in FORMULA_OVERRIDES:
        canonical_by_printed = {
            str(item.get("printed", "")).strip().casefold(): item["canonical"]
            for item in (product.get("ingredients") or [])
            if item.get("canonical")
        }
        canonical_by_printed["1,2-hexanediol"] = "1,2-HEXANEDIOL"
        canonical_by_printed["water"] = "AQUA"
        canonical_by_printed["aqua/water/eau"] = "AQUA"
        canonical_by_stripped = {
            re.sub(r"\s*\([^)]*\)", "", printed).strip().casefold(): canonical
            for printed, canonical in canonical_by_printed.items()
        }
        return [
            {
                "position": index,
                "printed": name,
                "canonical": (
                    canonical_by_printed.get(name.casefold())
                    or canonical_by_stripped.get(re.sub(r"\s*\([^)]*\)", "", name).strip().casefold())
                    or name.upper()
                ),
                "resolution": "resolved",
                "method": "retailer_formula_transcription",
                "confidence": 0.95,
            }
            for index, name in enumerate(FORMULA_OVERRIDES[key], start=1)
        ]

    unresolved_by_position = {
        int(item["position"]): item for item in (product.get("unresolved") or [])
    }
    items = sorted(product.get("ingredients") or [], key=lambda item: int(item["position"]))
    output: list[dict[str, Any]] = []
    index = 0
    while index < len(items):
        current = items[index]
        printed = str(current.get("printed", "")).strip()
        if (
            printed == "1"
            and index + 1 < len(items)
            and str(items[index + 1].get("printed", "")).strip().casefold() == "2-hexanediol"
        ):
            output.append({
                "position": len(output) + 1,
                "source_positions": [int(current["position"]), int(items[index + 1]["position"])],
                "printed": "1,2-Hexanediol",
                "canonical": "1,2-HEXANEDIOL",
                "resolution": "resolved",
                "method": "rejoined_split_inci_token",
                "confidence": 0.98,
            })
            index += 2
            continue
        item = ingredient_from_export(current, unresolved_by_position)
        item["position"] = len(output) + 1
        output.append(item)
        index += 1

    # Some exports keep unresolved entries separately even if no parsed token
    # was emitted. Preserve those in sequence without duplicating any position.
    seen_positions = {
        position
        for entry in output
        for position in entry.get("source_positions", [entry["position"]])
    }
    for pos, item in sorted(unresolved_by_position.items()):
        if pos in seen_positions:
            continue
        output.append({
            "position": len(output) + 1,
            "printed": str(item.get("printed", "")),
            "canonical": None,
            "resolution": "unresolved",
            "method": "unresolved",
            "confidence": 0,
            "reason": item.get("reason", "not resolved in the source import"),
        })
    return output


def may_contain_tokens(product: dict[str, Any]) -> list[dict[str, Any]]:
    tokens: list[dict[str, Any]] = []
    for index, item in enumerate(product.get("may_contain") or [], start=1):
        if isinstance(item, str):
            tokens.append({"position": index, "printed": item, "canonical": None})
        else:
            tokens.append({
                "position": index,
                "printed": str(item.get("printed", "")),
                "canonical": item.get("canonical"),
            })
    return tokens


def build_catalog(rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    products: list[dict[str, Any]] = []
    seen_ids: set[str] = set()
    for row in rows:
        product = row["product"]
        product_id = str(product["product_key"]).removeprefix("local:")
        if product_id in seen_ids:
            raise ValueError(f"Duplicate product id: {product_id}")
        seen_ids.add(product_id)

        ingredients = resolve_positions(product)
        source = source_record(SOURCE_OVERRIDES[product_id]) if product_id in SOURCE_OVERRIDES else source_record(product.get("source"))
        if ingredients and source is None:
            raise ValueError(f"Formula without source provenance: {product_id}")
        formula_status = (
            "unavailable" if not ingredients
            else "partial" if any(item["resolution"] == "unresolved" for item in ingredients)
            else "complete"
        )
        public_product: dict[str, Any] = {
            "id": PUBLIC_ID_OVERRIDES.get(product_id, product_id),
            "brand": product["brand"],
            "name": product["name"],
            "variant_status": product.get("variant_status", "unresolved"),
            "trust": product.get("trust", "unverified"),
            "formula_status": formula_status,
            "source": source,
            "ingredients": ingredients,
            "may_contain": may_contain_tokens(product),
        }
        if "size" in product and product["size"]:
            public_product["size"] = product["size"]
        if product_id == "pleva-aspintoxin-cream":
            public_product["variant_status"] = "confirmed"
            public_product["normalization_notes"] = [PLEVA_NOTE]
            public_product["supporting_sources"] = PLEVA_SUPPORTING_SOURCES
            for entry in public_product["ingredients"]:
                if entry["position"] == 13:
                    entry["printed"] = "Hdrogenated Olive Oil"
                    entry["method"] = "source_typo_correction"
                    entry["confidence"] = 0.98
                elif entry["position"] == 21:
                    entry["printed"] = "Ethylhexylglycerine"
                    entry["method"] = "source_typo_correction"
                    entry["confidence"] = 0.98
        if product_id == "vt-reedle-shot-700":
            # Product identity and strength were directly confirmed by the owner.
            public_product["variant_status"] = "confirmed"
        if product_id in SUPPORTING_SOURCES_BY_PRODUCT:
            public_product["supporting_sources"] = SUPPORTING_SOURCES_BY_PRODUCT[product_id]
        if product_id in NORMALIZATION_NOTES:
            public_product["normalization_notes"] = NORMALIZATION_NOTES[product_id]
        products.append(public_product)

    return sorted(products, key=lambda item: item["id"])


def validate_catalog(products: list[dict[str, Any]]) -> None:
    if len(products) != 69:
        raise ValueError(f"Expected 69 inventory products; found {len(products)}")

    forbidden_fields = {
        "shelf", "units_owned", "wishlist", "notes", "private_notes",
        "observation_id", "raw_capture", "exported_at",
    }

    def check_public_fields(value: Any, product_id: str) -> None:
        if isinstance(value, dict):
            leaked = forbidden_fields.intersection(value)
            if leaked:
                raise ValueError(f"Private field(s) {sorted(leaked)} in {product_id}")
            for child in value.values():
                check_public_fields(child, product_id)
        elif isinstance(value, list):
            for child in value:
                check_public_fields(child, product_id)

    for product in products:
        check_public_fields(product, product["id"])
        if product["formula_status"] != "unavailable" and not product["source"]:
            raise ValueError(f"Missing formula source for {product['id']}")
        if product["formula_status"] == "unavailable" and product["ingredients"]:
            raise ValueError(f"Unavailable formula has ingredients: {product['id']}")
        for position, ingredient in enumerate(product["ingredients"], start=1):
            if ingredient["position"] != position:
                raise ValueError(f"Non-contiguous ingredient order: {product['id']}")
            if not ingredient.get("printed"):
                raise ValueError(f"Ingredient without a printed name: {product['id']}")
            if ingredient["resolution"] == "unresolved" and ingredient.get("canonical") is not None:
                raise ValueError(f"Unresolved ingredient has a canonical name: {product['id']}")


def markdown(products: list[dict[str, Any]]) -> str:
    lines = [
        "# Product and ingredient catalog",
        "",
        "A curated public catalog of product names and ordered ingredient lists. "
        "It does not include ownership, shelf counts or wishlist data. See "
        "[`products.json`](products.json) for structured provenance and per-token confidence.",
        "",
        "Formula status is `complete` when every listed token is resolved, `partial` "
        "when unresolved source tokens remain, and `unavailable` when no formula "
        "was identified. `Variant uncertain` means the source formula may belong "
        "to a different formulation or product variant.",
        "",
    ]
    for product in sorted(products, key=lambda item: (item["brand"].casefold(), item["name"].casefold(), item["id"])):
        title = f"{product['brand']} — {product['name']}"
        lines.extend([f"## {title}", ""])
        lines.append(
            f"Variant: {product['variant_status']}; formula: {product['formula_status']}; "
            f"source confidence: {product['source']['confidence'] if product['source'] else 'n/a'}."
        )
        if product["source"]:
            lines.append(f"Source: [{product['source']['source']}](<{product['source']['url']}>).")
        if product.get("supporting_sources"):
            links = [
                f"[{source['source']}](<{source['url']}>)"
                for source in product["supporting_sources"]
            ]
            lines.append("Cross-checks: " + "; ".join(links) + ".")
        if product["formula_status"] == "unavailable":
            lines.append("Ingredients: formula not identified.")
        else:
            tokens = []
            for ingredient in product["ingredients"]:
                token = ingredient["printed"]
                if ingredient["resolution"] == "unresolved":
                    token = f"**UNRESOLVED:** {token}"
                elif ingredient["canonical"] and ingredient["canonical"].casefold() != token.casefold():
                    token = f"{token} ({ingredient['canonical']})"
                tokens.append(token)
            lines.append("Ordered ingredients: " + "; ".join(tokens) + ".")
        for note in product.get("normalization_notes", []):
            lines.append(f"Normalization: {note}")
        lines.append("")
    return "\n".join(lines)


def main() -> int:
    try:
        products = build_catalog(read_export())
        validate_catalog(products)
        OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
        payload = {
            "schema_version": 1,
            "license": "ODbL-1.0 / DbCL-1.0",
            "products": products,
        }
        JSON_OUTPUT.write_text(
            json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        MARKDOWN_OUTPUT.write_text(markdown(products), encoding="utf-8")
        print(f"Published {len(products)} products to {JSON_OUTPUT.relative_to(ROOT)} and {MARKDOWN_OUTPUT.relative_to(ROOT)}")
        print(f"Products with formulas: {sum(p['formula_status'] != 'unavailable' for p in products)}")
        print(f"Partial formulas: {sum(p['formula_status'] == 'partial' for p in products)}")
        print(f"Unavailable formulas: {sum(p['formula_status'] == 'unavailable' for p in products)}")
        return 0
    except (OSError, ValueError, KeyError, TypeError, json.JSONDecodeError) as error:
        print(f"publish_catalog: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
