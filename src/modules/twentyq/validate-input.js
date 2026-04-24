/**
 * @file Pre-AI input validator. Reject open-ended questions client-side
 * to save Workers AI Neurons. Accept only yes/no question shapes.
 *
 * Returns { ok: true, normalized } or { ok: false, reason }.
 * `normalized` strips extra whitespace + collapses internal runs to single
 * spaces. Original casing preserved (model handles capitalization fine).
 */

const MIN_LEN = 3;
const MAX_LEN = 200;

const OPEN_ENDED = /^\s*(what|how|why|which|who|where|when|tell me|describe|explain)\b/i;

/**
 * @typedef {{ ok: true, normalized: string } | { ok: false, reason: string }} ValidateResult
 */

/**
 * @param {string} raw
 * @returns {ValidateResult}
 */
export function validateQuestion(raw) {
  if (typeof raw !== "string") {
    return { ok: false, reason: "Please send a yes/no question after the command." };
  }
  const collapsed = raw.replace(/\s+/g, " ").trim();
  if (collapsed.length < MIN_LEN) {
    return {
      ok: false,
      reason: "Question too short — try something like <code>is it big?</code>.",
    };
  }
  if (collapsed.length > MAX_LEN) {
    return { ok: false, reason: `Question too long — keep it under ${MAX_LEN} characters.` };
  }
  if (OPEN_ENDED.test(collapsed)) {
    return {
      ok: false,
      reason: "Yes/no questions only — try <code>is it ...?</code> or <code>does it ...?</code>.",
    };
  }
  return { ok: true, normalized: collapsed };
}
