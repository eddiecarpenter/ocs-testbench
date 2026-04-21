#!/usr/bin/env node
/**
 * Extract a curated TAC catalogue from the Osmocom TAC database.
 *
 * Source: http://tacdb.osmocom.org/export/tacdb.json
 * Licence: CC-BY-SA v3.0 (© Harald Welte 2016) — attribution carried in
 * the header of the output file.
 *
 * The full database is ~9.6 MB and covers almost every cellular device
 * ever made. For the testbench we only need a plausible sample of
 * roughly the last 10 years of popular handsets, so this script picks
 * out a hand-curated subset by exact model-name match against the
 * `MODEL_PICKS` table below.
 *
 * Usage (from `web/`):
 *   curl -sSL http://tacdb.osmocom.org/export/tacdb.json -o /tmp/tacdb.json
 *   node scripts/extract-tac-catalog.mjs /tmp/tacdb.json \
 *        src/mocks/data/tacCatalog.json
 *
 * Re-run after adding/removing entries in `MODEL_PICKS` or when a fresh
 * snapshot of the upstream database is needed. The output is committed
 * to the repo — this script is not run at build time.
 */
import { readFileSync, writeFileSync } from 'node:fs';

/**
 * Hand-picked models per brand. Matched case-sensitively against the
 * upstream `brands[<brand>].models[<i>][<modelName>]` key. When a pick
 * resolves to more than one TAC in the upstream DB (common — different
 * regional SKUs get different TACs) the first one is taken. `year` is
 * editorial — the upstream DB doesn't expose a release year.
 */
/**
 * Note on coverage: the upstream Osmocom TAC DB is community-maintained
 * and its coverage of post-2019 flagships is patchy. The picks below are
 * limited to models for which the DB actually holds a TAC — enough for
 * a plausible testbench but not a comprehensive catalogue. Re-run this
 * script with a fresher snapshot if upstream data improves.
 */
const MODEL_PICKS = {
  Apple: [
    ['iPhone 5', 2012],
    ['iPhone 6', 2014],
    ['iPhone 6s', 2015],
    ['iPhone SE', 2016],
    ['iPhone 7', 2016],
    ['iPhone 7 Plus', 2016],
    ['iPhone 8', 2017],
    ['iPhone 13', 2021],
  ],
  Samsung: [
    ['Galaxy S5', 2014],
    ['Galaxy S7', 2016],
    ['Galaxy Note8', 2017],
    ['Galaxy Note 8 LTE', 2017],
  ],
  Google: [
    ['Pixel 2', 2017],
    ['Pixel 3', 2018],
    ['Pixel 4', 2019],
    ['Pixel 4a', 2020],
    ['Pixel XL', 2016],
  ],
  Xiaomi: [
    ['Mi A1', 2017],
    ['Mi 4i', 2015],
    ['Redmi Note 3', 2016],
    ['Redmi Note 7', 2019],
    ['Redmi 8A', 2019],
    ['Redmi 9A', 2020],
  ],
  Huawei: [
    ['P20 Pro', 2018],
    ['P20 lite', 2018],
    ['Mate 9', 2016],
  ],
  Sony: [
    ['Xperia XA2', 2018],
    ['Xperia Z3C', 2014],
  ],
  Motorola: [
    ['Moto G (4)', 2016],
    ['Moto G5', 2017],
    ['Moto G4 Plus', 2016],
    ['Moto G4 Play', 2016],
    ['Moto E', 2014],
  ],
};

const [, , inputPath, outputPath] = process.argv;
if (!inputPath || !outputPath) {
  console.error('Usage: extract-tac-catalog.mjs <osmocom-tacdb.json> <output.json>');
  process.exit(2);
}

const raw = JSON.parse(readFileSync(inputPath, 'utf8'));
if (!raw.brands) {
  console.error('Input does not look like an Osmocom TAC DB export (no `brands` key).');
  process.exit(1);
}

/** @type {{tac: string, manufacturer: string, model: string, year?: number}[]} */
const entries = [];
const missing = [];

for (const [brand, picks] of Object.entries(MODEL_PICKS)) {
  const brandEntry = raw.brands[brand];
  if (!brandEntry) {
    missing.push(`brand not found: ${brand}`);
    continue;
  }
  // Build a case-insensitive index so "Galaxy S23" matches whatever casing
  // the upstream uses (varies by brand).
  const index = new Map();
  for (const modelWrap of brandEntry.models) {
    for (const [modelName, modelInfo] of Object.entries(modelWrap)) {
      index.set(modelName.toLowerCase(), { modelName, modelInfo });
    }
  }
  for (const [wantedName, year] of picks) {
    const hit = index.get(wantedName.toLowerCase());
    if (!hit || !hit.modelInfo.tacs || hit.modelInfo.tacs.length === 0) {
      missing.push(`${brand} / ${wantedName}`);
      continue;
    }
    entries.push({
      tac: hit.modelInfo.tacs[0],
      manufacturer: brand,
      // Use whatever name the upstream DB carries — canonicalises casing.
      model: hit.modelName,
      year,
    });
  }
}

// Stable ordering: by manufacturer, then by year ascending, then by model.
entries.sort(
  (a, b) =>
    a.manufacturer.localeCompare(b.manufacturer) ||
    (a.year ?? 0) - (b.year ?? 0) ||
    a.model.localeCompare(b.model),
);

writeFileSync(
  outputPath,
  JSON.stringify(
    {
      $schema: {
        description:
          'Curated TAC catalogue extracted from the Osmocom TAC database.',
        source: 'http://tacdb.osmocom.org/export/tacdb.json',
        licence: 'CC-BY-SA v3.0 — © Harald Welte 2016',
        extractedAt: new Date().toISOString(),
        extractor: 'web/scripts/extract-tac-catalog.mjs',
      },
      entries,
    },
    null,
    2,
  ),
);

console.log(`Wrote ${entries.length} entries to ${outputPath}`);
if (missing.length > 0) {
  console.warn(`\nSkipped (not in upstream DB):\n  ${missing.join('\n  ')}`);
}
