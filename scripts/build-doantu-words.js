#!/usr/bin/env node
/**
 * @file build-doantu-words — fetches the middle-sized Vietnamese wordlist
 * (Viet22K) from duyet/vietnamese-wordlist and writes it to
 * src/modules/doantu/words-data.js as a static ES-module array.
 *
 * Source is an alphabetically-sorted Unicode dictionary, one word or phrase
 * per line. We normalize only (trim, lowercase, dedupe); NO length/char
 * filtering — ConceptNet's verify-and-fallback in api-client handles
 * unusable picks at round start.
 *
 * Source:  https://github.com/duyet/vietnamese-wordlist
 * Credits: Ho Ngoc Duc — Vietnamese word list (GPL).
 *
 * Usage:
 *   node scripts/build-doantu-words.js
 */

import { writeFileSync } from "node:fs";
import { resolve } from "node:path";

// Size options from the upstream repo: Viet11K / Viet22K / Viet39K / Viet74K.
// Viet22K is a good balance — enough variety, not overwhelmed by archaic terms.
const SOURCE_URL = "https://raw.githubusercontent.com/duyet/vietnamese-wordlist/master/Viet22K.txt";

const root = resolve(import.meta.dirname, "..");
const dst = resolve(root, "src/modules/doantu/words-data.js");

const res = await fetch(SOURCE_URL);
if (!res.ok) throw new Error(`fetch failed: ${res.status} ${res.statusText}`);
const text = await res.text();

// Normalize only: trim, lowercase, drop blanks, dedupe. Preserve source order.
// Multi-word entries keep their spaces — the api-client converts to underscore
// only at URL-build time, so the board still displays them naturally.
const words = Array.from(
  new Set(
    text
      .split(/\r?\n/)
      .map((w) => w.trim().toLowerCase())
      .filter((w) => w.length > 0),
  ),
);

if (words.length === 0) throw new Error("no words parsed from source");

// JSON.stringify each word — safer than manual quoting for Vietnamese
// diacritics and the occasional character that needs escaping.
const body = words.map((w) => `  ${JSON.stringify(w)},`).join("\n");
const out = [
  "// Auto-generated from https://github.com/duyet/vietnamese-wordlist (Viet22K.txt)",
  "// Credits: Ho Ngoc Duc — Vietnamese word list (GPL).",
  "// Normalized (lowercased + deduped) but otherwise unfiltered.",
  "// Regenerate with: node scripts/build-doantu-words.js",
  "export default [",
  body,
  "];",
  "",
].join("\n");

writeFileSync(dst, out);
console.log(`wrote ${dst} (${words.length} words)`);
