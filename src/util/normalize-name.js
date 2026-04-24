/**
 * @file normalize-name — case/space/punctuation-insensitive string folder.
 *
 * Shared across loldle-family modules so champion-name input ("Kai'Sa",
 * "kaisa", "KAI SA") collapses to the same comparable form.
 */

/**
 * @param {unknown} s — coerced to string.
 * @returns {string} lowercase, alphanumeric-only.
 */
export function normalize(s) {
  return String(s || "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");
}
