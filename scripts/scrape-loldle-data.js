#!/usr/bin/env node
/**
 * @file scrape-loldle-data — rebuilds src/modules/loldle/champions.json by
 * scraping loldle.net for canonical game fields (gender, positions,
 * range_type, regions, release_date) and merging with ddragon championFull
 * for display fields (title, resource, genre tags, skin count, sprite image).
 *
 * Writes both champions.json (authoring format) and champions-data.js (ESM
 * wrapper consumed by the bot). Replaces the hand-run build-loldle-data step.
 *
 * Source of truth — loldle.net embeds the full champion array in plaintext
 * inside its JS bundle at `<script src="js/index.<hash>.js">`, one record per
 * champion with the exact shape the bot needs. No CryptoJS decoding needed.
 *
 * Usage:
 *   node scripts/scrape-loldle-data.js
 *
 * Schedule: weekly via .github/workflows/scrape-loldle-data.yml
 */

import { writeFileSync } from "node:fs";
import { resolve } from "node:path";

const LOLDLE_CLASSIC = "https://loldle.net/classic";
const DDRAGON_VERSIONS = "https://ddragon.leagueoflegends.com/api/versions.json";
const ddragonChampUrl = (v) =>
  `https://ddragon.leagueoflegends.com/cdn/${v}/data/en_US/championFull.json`;

const LANE_MAP = {
  top: "top",
  jungle: "jungle",
  middle: "mid",
  bottom: "bottom",
  support: "support",
};

const GENDER_MAP = { male: "male", female: "female", other: "divers" };

const CHAMPION_RECORD_RX =
  /\{_id:"[a-f0-9]+",championId:"[^"]+",championName:"([^"]+)",gender:"([^"]+)",positions:\[([^\]]+)\],species:\[[^\]]+\],resource:"[^"]+",range_type:\[([^\]]+)\],regions:\[([^\]]+)\],release_date:"(\d{4})-\d{2}-\d{2}"\}/g;

async function fetchText(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`fetch ${url}: ${res.status} ${res.statusText}`);
  return res.text();
}

async function fetchJson(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`fetch ${url}: ${res.status} ${res.statusText}`);
  return res.json();
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
    const [, name, gender, positionsRaw, rangeTypeRaw, regionsRaw, year] = m;
    if (seen.has(name)) continue;
    seen.add(name);

    const lanes = parseJsArrayStrings(positionsRaw)
      .map((p) => LANE_MAP[p.toLowerCase()])
      .filter(Boolean);
    const regions = parseJsArrayStrings(regionsRaw).map(normalizeRegion);
    const rangeType = parseJsArrayStrings(rangeTypeRaw)[0]?.toLowerCase();

    records.push({
      name,
      gender: GENDER_MAP[gender.toLowerCase()] ?? "divers",
      attackType: rangeType === "melee" ? "close" : "range",
      lane: lanes.join(","),
      region: regions.join(","),
      releaseDate: Number(year),
    });
  }

  if (records.length === 0) {
    throw new Error(
      "loldle.net: zero champion records parsed — bundle format changed, update CHAMPION_RECORD_RX",
    );
  }
  return records;
}

async function fetchDdragon() {
  const versions = await fetchJson(DDRAGON_VERSIONS);
  const version = versions[0];
  const full = await fetchJson(ddragonChampUrl(version));
  return { version, champions: full.data };
}

function mergeRecords(loldleRecords, ddragonChampions) {
  const byName = new Map(loldleRecords.map((r) => [r.name, r]));
  const merged = [];
  const missing = [];

  for (const champ of Object.values(ddragonChampions)) {
    const lol = byName.get(champ.name);
    if (!lol) {
      missing.push(champ.name);
      continue;
    }
    merged.push({
      id: champ.id,
      name: champ.name,
      title: champ.title,
      resource: champ.partype,
      genre: champ.tags.join(","),
      skinCount: champ.skins.length,
      image: champ.image,
      gender: lol.gender,
      attackType: lol.attackType,
      releaseDate: lol.releaseDate,
      region: lol.region,
      lane: lol.lane,
    });
  }

  if (missing.length > 0) {
    console.warn(
      `warn: ${missing.length} ddragon champions absent from loldle.net (likely just-released): ${missing.join(", ")}`,
    );
  }

  merged.sort((a, b) => a.name.localeCompare(b.name));
  return merged;
}

const root = resolve(import.meta.dirname, "..");
const jsonPath = resolve(root, "src/modules/loldle/champions.json");
const esmPath = resolve(root, "src/modules/loldle/champions-data.js");

console.log("scraping loldle.net…");
const loldleRecords = await scrapeLoldle();
console.log(`  parsed ${loldleRecords.length} champions from loldle.net`);

console.log("fetching ddragon championFull…");
const { version, champions } = await fetchDdragon();
console.log(`  ddragon ${version}: ${Object.keys(champions).length} champions`);

const merged = mergeRecords(loldleRecords, champions);
console.log(`merged ${merged.length} champions`);

const json = JSON.stringify(merged, null, 4);
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
