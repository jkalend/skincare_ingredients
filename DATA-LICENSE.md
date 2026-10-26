# Data licence

This file covers the contents of `data/` in this repository. It does not cover
the source code, which is licensed separately.

## Open Beauty Facts derivative — ODbL 1.0

Product, brand, barcode and ingredient-list data derived from
[Open Beauty Facts](https://world.openbeautyfacts.org/) is a derivative database
under the [Open Database License (ODbL) 1.0](https://opendatacommons.org/licenses/odbl/1-0/).
The individual contents are under the
[Database Contents License (DbCL) 1.0](https://opendatacommons.org/licenses/dbcl/1.0/).

Attribution: **Contains data from Open Beauty Facts contributors, available
under the Open Database License (ODbL) 1.0.**

If you publicly use an adapted version of this database, you must also offer
that adapted database under the ODbL and keep the attribution. Do not add
technical measures that restrict the database unless you also distribute a
version without them.

## CosIng derivative — CC BY 4.0

Annex II–VI data derived from the European Commission's
[CosIng](https://ec.europa.eu/growth/tools-databases/cosing/) database is reused
under the Commission's reuse policy (Commission Decision 2011/833/EU), which
licenses Commission-owned content under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).

Attribution: **CosIng, © European Union, 1995–2026. Annexes II–VI of Regulation
(EC) No 1223/2009, retrieved from the European Commission CosIng API. Reused
under Commission Decision 2011/833/EU (CC BY 4.0). Changes: CSV rows transformed
into JSON; export metadata removed from the content hash.**

## Optional public shelf

`data/shelf/` is optional. It is a list of product keys/names, published as the
owner's personal metadata, not as an Open Beauty Facts derivative. If you do not
want the shelf public, keep it in gitignored `local/shelf/`.

## Excluded: scraped panels and personal data

Scraped ingredient panels, raw HTML, images, cookies, the personal shelf and
the HTTP cache are **not** part of `data/` and are not published. They live in
gitignored `local/` with provenance, for personal use only. Their source terms
may forbid redistribution; do not commit them.

## No legal advice

This file describes the licences of the data sources. It is not legal advice.
If you republish the database, verify the current terms at the source.
