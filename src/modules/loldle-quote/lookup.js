/**
 * @file Champion lookup over the quote pool — case/space/punctuation-
 * insensitive with unique-prefix fallback, matching classic's behaviour.
 */

import { normalize } from "../../util/normalize-name.js";

export function findChampion(pool, input) {
  const q = normalize(input);
  if (!q) return null;

  const exact = pool.find((c) => normalize(c.championName) === q);
  if (exact) return exact;

  const prefix = pool.filter((c) => normalize(c.championName).startsWith(q));
  return prefix.length === 1 ? prefix[0] : null;
}
