/**
 * @file escape-html — minimal HTML entity escaper for Telegram HTML parse mode.
 *
 * Telegram's HTML parse mode needs only four characters escaped: &, <, >, ".
 * Keep this as a tiny hand-rolled function — pulling in a library for four
 * replacements is YAGNI.
 *
 * @see https://core.telegram.org/bots/api#html-style
 */

/**
 * @param {unknown} s — coerced to string.
 * @returns {string}
 */
export function escapeHtml(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}
