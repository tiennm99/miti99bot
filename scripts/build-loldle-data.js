#!/usr/bin/env node
/**
 * @file build-loldle-data — wraps src/modules/loldle/champions.json
 * as an ES module export to sidestep Node/esbuild disagreement on
 * JSON import attributes (Node 24 requires `with { type: "json" }`,
 * wrangler's bundled esbuild doesn't parse it yet).
 *
 * Run after any update to champions.json.
 */

import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const src = resolve(root, "src/modules/loldle/champions.json");
const dst = resolve(root, "src/modules/loldle/champions-data.js");

const json = readFileSync(src, "utf8").trimEnd();
const out = [
  "// Auto-generated from champions.json — do NOT edit by hand.",
  "// Regenerate with: node scripts/build-loldle-data.js",
  `export default ${json};`,
  "",
].join("\n");

writeFileSync(dst, out);
console.log(`wrote ${dst}`);
