#!/usr/bin/env node
/**
 * @file build-semantle-words — fetches the google-10000-english (no-swears)
 * word list and writes it verbatim (lowercased + deduped, no filtering) to
 * src/modules/semantle/words-data.js.
 *
 * The list is sorted by Google Ngram frequency. ConceptNet's verify-and-
 * fallback in api-client.js handles weird picks (`a`, `dvd`, etc.) by
 * rejecting them at round-start if they have no concept edges, so there's
 * no need to pre-filter here.
 *
 * Source:  https://github.com/first20hours/google-10000-english
 * Credits: Josh Kaufman (first20hours) — list derived from Peter Norvig's
 *          Google Ngram analysis.
 *
 * Usage:
 *   node scripts/build-semantle-words.js
 */

import { writeFileSync } from "node:fs";
import { resolve } from "node:path";

const SOURCE_URL =
  "https://raw.githubusercontent.com/first20hours/google-10000-english/master/google-10000-english-no-swears.txt";

const root = resolve(import.meta.dirname, "..");
const dst = resolve(root, "src/modules/semantle/words-data.js");

const res = await fetch(SOURCE_URL);
if (!res.ok) throw new Error(`fetch failed: ${res.status} ${res.statusText}`);
const text = await res.text();

// Normalize only: trim whitespace, lowercase, drop blanks, dedupe.
// Preserve original frequency order — source is ranked by Google Ngram.
const words = Array.from(
  new Set(
    text
      .split(/\r?\n/)
      .map((w) => w.trim().toLowerCase())
      .filter((w) => w.length > 0),
  ),
);

if (words.length === 0) throw new Error("no words parsed from source");

const body = words.map((w) => `  "${w}",`).join("\n");
const out = [
  "// Auto-generated from https://github.com/first20hours/google-10000-english",
  "// Credits: Josh Kaufman (first20hours) — common English words by Google Ngram frequency.",
  "// Normalized (lowercased, trimmed, deduped) but otherwise unfiltered.",
  "// Regenerate with: node scripts/build-semantle-words.js",
  "export default [",
  body,
  "];",
  "",
].join("\n");

writeFileSync(dst, out);
console.log(`wrote ${dst} (${words.length} words)`);
