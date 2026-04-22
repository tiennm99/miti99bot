#!/usr/bin/env node
/**
 * @file Rebuilds src/modules/loldle/champions.json from loldle.net's JS
 * bundle. The bundle embeds the full champion array in plaintext. Records
 * have two shapes (older champions carry _id/championId, newer ones don't);
 * both shapes share the gameplay fields: championName, gender, positions,
 * species, resource, range_type, regions, release_date — the only fields
 * the bot consumes, so we keep just those.
 *
 * The bot imports the resulting JSON directly via `with { type: "json" }`.
 *
 * Usage:   node scripts/scrape-loldle-data.js
 * Schedule: weekly via .github/workflows/scrape-loldle-data.yml
 */

import { writeFileSync } from "node:fs";
import { resolve } from "node:path";

const LOLDLE_CLASSIC = "https://loldle.net/classic";

// _id and championId are only present on older records — make them optional
// and discard them (they're not used downstream).
const CHAMPION_RECORD_RX =
  /\{(?:_id:"[a-f0-9]+",championId:"[^"]+",)?championName:"([^"]+)",gender:"([^"]+)",positions:\[([^\]]+)\],species:\[([^\]]+)\],resource:"([^"]+)",range_type:\[([^\]]+)\],regions:\[([^\]]+)\],release_date:"(\d{4}-\d{2}-\d{2})"\}/g;

async function fetchText(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`fetch ${url}: ${res.status} ${res.statusText}`);
  return res.text();
}

function parseJsArrayStrings(inner) {
  return [...inner.matchAll(/"([^"]+)"/g)].map((m) => m[1]);
}

async function scrapeLoldle() {
  const html = await fetchText(LOLDLE_CLASSIC);
  const scriptMatch = html.match(/<script\s+src="(js\/index\.[^"]+\.js)"/);
  if (!scriptMatch) throw new Error("loldle.net: could not locate index.js script tag in HTML");
  const bundleUrl = `https://loldle.net/${scriptMatch[1]}`;
  const bundle = await fetchText(bundleUrl);

  const seen = new Set();
  const records = [];
  for (const m of bundle.matchAll(CHAMPION_RECORD_RX)) {
    const [, championName, gender, positions, species, resource, rangeType, regions, releaseDate] =
      m;
    if (seen.has(championName)) continue;
    seen.add(championName);

    records.push({
      championName,
      gender,
      positions: parseJsArrayStrings(positions),
      species: parseJsArrayStrings(species),
      resource,
      range_type: parseJsArrayStrings(rangeType),
      regions: parseJsArrayStrings(regions),
      release_date: releaseDate,
    });
  }

  if (records.length === 0) {
    throw new Error(
      "loldle.net: zero champion records parsed — bundle format changed, update CHAMPION_RECORD_RX",
    );
  }
  records.sort((a, b) => a.championName.localeCompare(b.championName));
  return records;
}

const root = resolve(import.meta.dirname, "..");
const jsonPath = resolve(root, "src/modules/loldle/champions.json");

console.log("scraping loldle.net…");
const records = await scrapeLoldle();
console.log(`  parsed ${records.length} champions`);

writeFileSync(jsonPath, `${JSON.stringify(records, null, 4)}\n`);
console.log(`wrote ${jsonPath}`);
