/**
 * @file Workers AI client — wraps env.AI.run for the twentyq judge.
 * Uses traditional function calling on @cf/google/gemma-4-26b-a4b-it.
 *
 * Returns a structured { is_guess, answer, hint } per turn. On any AI
 * failure, throws UpstreamError so handlers can show a friendly retry
 * message instead of a 500.
 *
 * Defensive: tolerates both Cloudflare's "traditional" tool-call shape
 * ({ name, arguments }) and OpenAI-style ({ function: { name, arguments } }).
 */

import { ANSWER_FUNCTION_SCHEMA, buildSystemPrompt } from "./prompts.js";

export const MODEL_ID = "@cf/google/gemma-4-26b-a4b-it";

const DEFAULT_FALLBACK = {
  is_guess: false,
  answer: "no",
  hint: "I couldn't fully parse that — try a clear yes/no question.",
};

export class UpstreamError extends Error {
  /** @param {string} message @param {{ cause?: unknown, status?: number }} [opts] */
  constructor(message, { cause, status } = {}) {
    super(message);
    this.name = "UpstreamError";
    this.cause = cause;
    this.status = status;
  }
}

/**
 * Extract the structured tool-call payload from a Workers AI response.
 * Handles both shapes: `tool_calls[].arguments` (traditional) and
 * `tool_calls[].function.arguments` (OpenAI-style, possibly stringified).
 *
 * @param {any} response
 * @returns {object | null}
 */
export function parseToolCall(response) {
  const calls = response?.tool_calls;
  if (!Array.isArray(calls) || calls.length === 0) return null;
  const first = calls[0];
  if (!first) return null;

  // Cloudflare "traditional" shape: { name, arguments: { ... } }
  if (first.arguments && typeof first.arguments === "object") {
    return first.arguments;
  }
  // OpenAI-style: { function: { name, arguments: "..." | { ... } } }
  const fnArgs = first.function?.arguments;
  if (fnArgs && typeof fnArgs === "object") return fnArgs;
  if (typeof fnArgs === "string") {
    try {
      return JSON.parse(fnArgs);
    } catch {
      return null;
    }
  }
  // Some models return stringified arguments at the top level too.
  if (typeof first.arguments === "string") {
    try {
      return JSON.parse(first.arguments);
    } catch {
      return null;
    }
  }
  return null;
}

/**
 * Coerce any tool-call payload into the canonical { is_guess, answer, hint }
 * shape, applying defaults if fields are missing or malformed.
 *
 * @param {any} payload
 * @returns {{ is_guess: boolean, answer: "yes"|"no", hint: string }}
 */
export function normalizeJudgement(payload) {
  if (!payload || typeof payload !== "object") return { ...DEFAULT_FALLBACK };
  const is_guess = payload.is_guess === true;
  const answerLower = String(payload.answer ?? "").toLowerCase();
  const answer = answerLower === "yes" ? "yes" : "no";
  const hint =
    typeof payload.hint === "string" && payload.hint.trim().length > 0
      ? payload.hint.trim()
      : DEFAULT_FALLBACK.hint;
  return { is_guess, answer, hint };
}

/**
 * Strip any case-insensitive substring of the secret from the hint.
 * Defense-in-depth — the system prompt forbids it but we don't trust the model.
 *
 * @param {string} hint
 * @param {string} target
 * @returns {string}
 */
export function redactSecret(hint, target) {
  if (!target) return hint;
  const escaped = target.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const re = new RegExp(`\\b${escaped}\\b`, "ig");
  const out = hint.replace(re, "(redacted)");
  return out.length > 0 ? out : "the hint was redacted to avoid revealing the answer";
}

/**
 * Judge a single user turn.
 *
 * @param {{ AI: { run: (model: string, body: object) => Promise<any> } }} env
 * @param {import("./state.js").TwentyqGameState} state
 * @param {string} userInput  — already validated/normalized by validate-input.js
 * @returns {Promise<{ is_guess: boolean, answer: "yes"|"no", hint: string }>}
 */
export async function judge(env, state, userInput) {
  if (!env?.AI?.run) {
    throw new UpstreamError("Workers AI binding not available");
  }
  const messages = [
    { role: "system", content: buildSystemPrompt(state) },
    { role: "user", content: userInput },
  ];
  let response;
  try {
    response = await env.AI.run(MODEL_ID, {
      messages,
      tools: [ANSWER_FUNCTION_SCHEMA],
      temperature: 0.3,
    });
  } catch (err) {
    throw new UpstreamError("env.AI.run threw", { cause: err });
  }
  const payload = parseToolCall(response);
  const judgement = normalizeJudgement(payload);
  judgement.hint = redactSecret(judgement.hint, state.target);
  return judgement;
}
