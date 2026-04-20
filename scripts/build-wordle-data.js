#!/usr/bin/env node
/**
 * @file build-wordle-data — fetches the 5-letter wordle word list from the
 * dracos gist and writes it as an ES module export to
 * src/modules/wordle/words-data.js.
 *
 * Source: https://gist.github.com/dracos/dd0668f281e685bad51479e5acaadb93
 * Credits: Anna Eilering (dracos) — combined Wordle dictionary of allowed guesses.
 *
 * Usage:
 *   node scripts/build-wordle-data.js
 */

import { writeFileSync } from "node:fs";
import { resolve } from "node:path";

const GIST_URL = "https://gist.githubusercontent.com/dracos/dd0668f281e685bad51479e5acaadb93/raw/";

const root = resolve(import.meta.dirname, "..");
const dst = resolve(root, "src/modules/wordle/words-data.js");

const res = await fetch(GIST_URL);
if (!res.ok) throw new Error(`fetch failed: ${res.status} ${res.statusText}`);
const text = await res.text();

const words = [
  ...new Set(
    text
      .split(/\r?\n/)
      .map((w) => w.trim().toLowerCase())
      .filter((w) => /^[a-z]{5}$/.test(w)),
  ),
].sort();

if (words.length === 0) throw new Error("no words parsed from gist");

const body = words.map((w) => `  "${w}",`).join("\n");
const out = [
  "// Auto-generated from https://gist.github.com/dracos/dd0668f281e685bad51479e5acaadb93",
  "// Credits: Anna Eilering (dracos) — combined Wordle dictionary of allowed 5-letter guesses.",
  "// Regenerate with: node scripts/build-wordle-data.js",
  "export default [",
  body,
  "];",
  "",
].join("\n");

writeFileSync(dst, out);
console.log(`wrote ${dst} (${words.length} words)`);
