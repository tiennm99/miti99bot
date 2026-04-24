#!/usr/bin/env node
/**
 * @file Builds emoji + quote pools for loldle-emoji and loldle-quote modules.
 *
 * Data sources:
 *   - classic's src/modules/loldle/champions.json (already scraped) — feeds
 *     the algorithmic emoji derivation (species/region/resource mapping).
 *   - Data Dragon latest patch — fetched once for champion `title` + `lore`
 *     blurb, used to seed quote text.
 *
 * Why algorithmic emoji instead of scrape:
 *   loldle.net's JS bundle contains zero emoji code points — emoji sequences
 *   are stored encrypted (daily rotation only) in cache.loldle.net. Scraping
 *   a full pool from them is not feasible. We derive a loldle-style
 *   3-emoji sequence from champion metadata (gender/species/region/resource/
 *   position). The mapping is handcrafted but deterministic.
 *
 * Why lore-blurb for quote:
 *   Voice-line transcripts are not in a public official feed. Champion
 *   `title` (e.g. "the Nine-Tailed Fox") + first sentence of `lore` gives
 *   recognizable per-champion text for all 165+ champions.
 *
 * Usage: node scripts/fetch-ddragon-data.js
 */

import { mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import championsData from "../src/modules/loldle/champions.json" with { type: "json" };

// ───── emoji mapping tables ─────
// Each category yields one emoji. A champion's sequence picks the strongest
// signal from species → region → position + resource/range. Short (3 emojis)
// by design — loldle.net typically shows 3.

const SPECIES_EMOJI = {
  Yordle: "🧚",
  Darkin: "⚔️",
  Demon: "😈",
  Dragon: "🐉",
  Cat: "🐱",
  Dog: "🐕",
  "Void-Being": "👁️",
  Void: "👁️",
  Undead: "💀",
  Spirit: "👻",
  Celestial: "🌟",
  Aspect: "🌟",
  God: "⚜️",
  "God-Warrior": "⚜️",
  Yeti: "❄️",
  Troll: "🧌",
  Minotaur: "🐂",
  Cyborg: "🤖",
  Golem: "🗿",
  Plant: "🌱",
  Rat: "🐀",
  Revenant: "👻",
  Iceborn: "🥶",
  Vastayan: "🦊",
  Brackern: "🦂",
  "Magically Altered": "✨",
  Magicborn: "🔮",
  "Chemically Altered": "🧪",
  Baccai: "💀",
  Spiritualist: "👻",
  Human: "🧝",
  Unknown: "❓",
};

const REGION_EMOJI = {
  Demacia: "🛡️",
  Noxus: "🗡️",
  Shurima: "🏜️",
  Freljord: "❄️",
  Ionia: "🏯",
  Piltover: "⚙️",
  Zaun: "🧪",
  "Bandle City": "🏡",
  Bilgewater: "⚓",
  "Shadow Isles": "🌫️",
  Targon: "⛰️",
  Ixtal: "🌿",
  Void: "👾",
  Icathia: "👾",
  Camavor: "⚜️",
  Runeterra: "🌍",
};

const RESOURCE_EMOJI = {
  Mana: "🔮",
  Manaless: "💪",
  Energy: "⚡",
  Fury: "💢",
  Rage: "😡",
  Ferocity: "🔥",
  Bloodthirst: "🩸",
  Flow: "💧",
  Heat: "🔥",
  Courage: "🦁",
  Grit: "🪨",
  Shield: "🛡️",
  "Health costs": "❤️‍🩹",
};

const POSITION_EMOJI = {
  Top: "⛰️",
  Middle: "✨",
  Jungle: "🌲",
  Bottom: "🏹",
  Support: "💕",
};

function pickEmoji(table, keys, fallback = "") {
  for (const k of keys) if (table[k]) return table[k];
  return fallback;
}

function deriveEmoji(champ) {
  const species = pickEmoji(SPECIES_EMOJI, champ.species ?? [], "");
  const region = pickEmoji(REGION_EMOJI, champ.regions ?? [], "🌍");
  const resource = RESOURCE_EMOJI[champ.resource] ?? "";
  const position = pickEmoji(POSITION_EMOJI, champ.positions ?? [], "");

  // always 3 emojis — drop the weakest signals if we overflow
  const parts = [species, region, resource || position].filter(Boolean);
  if (parts.length < 3) parts.push(position || "🎮");
  return parts.slice(0, 3).join(" ");
}

// ───── DDragon champion name mapping ─────

function ddragonKey(championName) {
  // DDragon uses PascalCase, but only capitalizes the first letter of the
  // first word — e.g. "Kai'Sa" → "Kaisa", "Cho'Gath" → "Chogath".
  // Multi-word names keep internal capitalization: "Miss Fortune" → "MissFortune".
  const overrides = {
    Wukong: "MonkeyKing",
    "Nunu & Willump": "Nunu",
    "Renata Glasc": "Renata",
    "Dr. Mundo": "DrMundo",
  };
  if (overrides[championName]) return overrides[championName];
  // Strip apostrophes/periods within a word (fold "Kai'Sa" → "KaiSa" → "Kaisa").
  // If the name has no internal spaces but has an apostrophe/period, lowercase
  // everything after the first letter: "Kai'Sa" → "K" + "aisa".
  const stripped = championName.replace(/['.]/g, "");
  if (!stripped.includes(" ") && !stripped.includes("&")) {
    return stripped[0].toUpperCase() + stripped.slice(1).toLowerCase();
  }
  return stripped.replace(/[\s&]/g, "").replace(/^(.)/, (c) => c.toUpperCase());
}

async function fetchJson(url) {
  const r = await fetch(url);
  if (!r.ok) throw new Error(`fetch ${url}: ${r.status} ${r.statusText}`);
  return r.json();
}

function firstSentence(text) {
  if (!text) return "";
  const clean = text
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  const m = clean.match(/^[^.!?]+[.!?]/);
  return (m ? m[0] : clean).trim();
}

async function buildQuotes() {
  const versions = await fetchJson("https://ddragon.leagueoflegends.com/api/versions.json");
  const v = versions[0];
  console.log(`  DDragon version: ${v}`);
  const summary = await fetchJson(
    `https://ddragon.leagueoflegends.com/cdn/${v}/data/en_US/champion.json`,
  );
  const summaryByKey = summary.data;
  // Secondary lookup: normalize(ddragonKey) → actual key. Handles DDragon's
  // inconsistent casing ("Kaisa" vs "KSante" vs "KogMaw") by folding to
  // lowercase-alphanumeric for comparison.
  const normMap = new Map();
  for (const k of Object.keys(summaryByKey)) {
    normMap.set(k.toLowerCase().replace(/[^a-z0-9]/g, ""), k);
  }

  const out = [];
  const missing = [];
  for (const champ of championsData) {
    const attempted = ddragonKey(champ.championName);
    let entry = summaryByKey[attempted];
    if (!entry) {
      const norm = attempted.toLowerCase().replace(/[^a-z0-9]/g, "");
      const realKey = normMap.get(norm);
      if (realKey) entry = summaryByKey[realKey];
    }
    if (!entry) {
      missing.push(`${champ.championName} → ${attempted}`);
      continue;
    }
    // Strip the champion's own name from the quote so it isn't a giveaway.
    // Handle first names too ("Ahri" in "Ahri is a fox-like vastaya").
    const nameParts = new Set(
      [champ.championName, ...champ.championName.split(/[\s'.]+/)].filter(
        (s) => s && s.length >= 3,
      ),
    );
    const nameRx = new RegExp(`\\b(${[...nameParts].join("|")})\\b`, "gi");
    const sentence = firstSentence(entry.blurb).replace(nameRx, "___");
    out.push({
      championName: champ.championName,
      // Title already includes "the" prefix: "the Nine-Tailed Fox — …"
      quote: `${entry.title} — ${sentence}`,
    });
  }
  if (missing.length) {
    console.warn(`  WARN: ${missing.length} champions missing from DDragon:`);
    for (const m of missing) console.warn(`    ${m}`);
  }
  return out;
}

// ───── main ─────

const root = resolve(import.meta.dirname, "..");

console.log("deriving emoji sequences from champion metadata…");
const emojis = championsData
  .map((c) => ({ championName: c.championName, emojis: deriveEmoji(c) }))
  .sort((a, b) => a.championName.localeCompare(b.championName));
const emojisPath = resolve(root, "src/modules/loldle-emoji/emojis.json");
mkdirSync(resolve(emojisPath, ".."), { recursive: true });
writeFileSync(emojisPath, `${JSON.stringify(emojis, null, 4)}\n`);
console.log(`wrote ${emojisPath} (${emojis.length} champions)`);

console.log("fetching DDragon for quote text…");
const quotes = (await buildQuotes()).sort((a, b) => a.championName.localeCompare(b.championName));
const quotesPath = resolve(root, "src/modules/loldle-quote/quotes.json");
mkdirSync(resolve(quotesPath, ".."), { recursive: true });
writeFileSync(quotesPath, `${JSON.stringify(quotes, null, 4)}\n`);
console.log(`wrote ${quotesPath} (${quotes.length} champions)`);
