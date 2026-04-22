#!/usr/bin/env node
/**
 * @file scrape-loldle-data — rebuilds src/modules/loldle/champions.json from
 * loldle.net's JS bundle, the canonical source for the classic-mode axes:
 * gender, species, resource, attackType, region, lane, releaseDate.
 *
 * loldle.net embeds the full champion array in plaintext inside its JS bundle
 * at `<script src="js/index.<hash>.js">`, one record per champion with the
 * exact shape the bot needs. No CryptoJS decoding, no ddragon merge.
 *
 * Writes both champions.json (authoring format) and champions-data.js (ESM
 * wrapper consumed by the bot).
 *
 * Usage:
 *   node scripts/scrape-loldle-data.js
 *
 * Schedule: weekly via .github/workflows/scrape-loldle-data.yml
 */

import { writeFileSync } from "node:fs";
import { resolve } from "node:path";

const LOLDLE_CLASSIC = "https://loldle.net/classic";

const LANE_MAP = {
  top: "top",
  jungle: "jungle",
  middle: "mid",
  bottom: "bottom",
  support: "support",
};

const GENDER_MAP = { male: "male", female: "female", other: "divers" };

const CHAMPION_RECORD_RX =
  /\{_id:"[a-f0-9]+",championId:"[^"]+",championName:"([^"]+)",gender:"([^"]+)",positions:\[([^\]]+)\],species:\[([^\]]+)\],resource:"([^"]+)",range_type:\[([^\]]+)\],regions:\[([^\]]+)\],release_date:"(\d{4})-\d{2}-\d{2}"\}/g;

async function fetchText(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`fetch ${url}: ${res.status} ${res.statusText}`);
  return res.text();
}

function parseJsArrayStrings(inner) {
  return [...inner.matchAll(/"([^"]+)"/g)].map((m) => m[1]);
}

function normalizeRegion(name) {
  return name.toLowerCase().replace(/\s+/g, "-");
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
    const [, name, gender, positionsRaw, speciesRaw, resource, rangeTypeRaw, regionsRaw, year] = m;
    if (seen.has(name)) continue;
    seen.add(name);

    const lanes = parseJsArrayStrings(positionsRaw)
      .map((p) => LANE_MAP[p.toLowerCase()])
      .filter(Boolean);
    const regions = parseJsArrayStrings(regionsRaw).map(normalizeRegion);
    const species = parseJsArrayStrings(speciesRaw).map((s) => s.toLowerCase());
    const rangeType = parseJsArrayStrings(rangeTypeRaw)[0]?.toLowerCase();

    records.push({
      id: name,
      name,
      gender: GENDER_MAP[gender.toLowerCase()] ?? "divers",
      species: species.join(","),
      resource,
      attackType: rangeType === "melee" ? "close" : "range",
      region: regions.join(","),
      lane: lanes.join(","),
      releaseDate: Number(year),
    });
  }

  if (records.length === 0) {
    throw new Error(
      "loldle.net: zero champion records parsed — bundle format changed, update CHAMPION_RECORD_RX",
    );
  }
  records.sort((a, b) => a.name.localeCompare(b.name));
  return records;
}

const root = resolve(import.meta.dirname, "..");
const jsonPath = resolve(root, "src/modules/loldle/champions.json");
const esmPath = resolve(root, "src/modules/loldle/champions-data.js");

console.log("scraping loldle.net…");
const records = await scrapeLoldle();
console.log(`  parsed ${records.length} champions`);

const json = JSON.stringify(records, null, 4);
writeFileSync(jsonPath, `${json}\n`);
console.log(`wrote ${jsonPath}`);

const esm = [
  "// Auto-generated from champions.json — do NOT edit by hand.",
  "// Regenerate with: node scripts/scrape-loldle-data.js",
  `export default ${json};`,
  "",
].join("\n");
writeFileSync(esmPath, esm);
console.log(`wrote ${esmPath}`);
