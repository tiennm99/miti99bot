#!/usr/bin/env node
/**
 * @file build-semantle-words — fetches a common-English word list (10k
 * by Google Ngram frequency, curated by first20hours) and writes the
 * 4–10 letter alphabetic subset to src/modules/semantle/words-data.js.
 *
 * Why this list:
 *   - Sorted by frequency → easy top-N trimming to drop function words
 *     (`the`, `of`, `and`) that make terrible guessing targets.
 *   - Already de-swear-ed. No further blocklist needed.
 *   - Tiny (~90 KB raw, ~30 KB gzipped) — comfortably inside the Worker
 *     size budget.
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

// Skip the top-N most frequent words — too common, lousy puzzles.
const SKIP_TOP_N = 200;
const MIN_LEN = 4;
const MAX_LEN = 10;

const root = resolve(import.meta.dirname, "..");
const dst = resolve(root, "src/modules/semantle/words-data.js");

const res = await fetch(SOURCE_URL);
if (!res.ok) throw new Error(`fetch failed: ${res.status} ${res.statusText}`);
const text = await res.text();

const lines = text.split(/\r?\n/).map((w) => w.trim().toLowerCase());

// Preserve original frequency order while filtering — downstream consumers
// can still sample uniformly, but the index itself stays a frequency rank.
const words = Array.from(
  new Set(
    lines
      .slice(SKIP_TOP_N)
      .filter((w) => w.length >= MIN_LEN && w.length <= MAX_LEN && /^[a-z]+$/.test(w)),
  ),
);

if (words.length === 0) throw new Error("no words parsed from source");

const body = words.map((w) => `  "${w}",`).join("\n");
const out = [
  "// Auto-generated from https://github.com/first20hours/google-10000-english",
  "// Credits: Josh Kaufman (first20hours) — common English words by Google Ngram frequency.",
  `// Filter: ${MIN_LEN}–${MAX_LEN} ASCII letters, skip top ${SKIP_TOP_N} most common.`,
  "// Regenerate with: node scripts/build-semantle-words.js",
  "export default [",
  body,
  "];",
  "",
].join("\n");

writeFileSync(dst, out);
console.log(`wrote ${dst} (${words.length} words)`);
