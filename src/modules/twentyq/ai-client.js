/**
 * @file Workers AI client — wraps env.AI.run for the twentyq judge.
 *
 * Plain JSON-in-content approach: the system prompt instructs the model to
 * emit one-line JSON, we grep it out of the response. No function calling,
 * no tools array — maximum compatibility across Workers AI models.
 *
 * On any AI failure or unparseable output, throws UpstreamError / returns
 * defensive defaults so handlers can show a friendly retry message.
 *
 * Rich logging on failure paths so wrangler tail surfaces the actual cause.
 */

import { buildStartRoundPrompt, buildSystemPrompt } from "./prompts.js";

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
 * Pull the response text out of whatever shape env.AI.run returned.
 * Handles { response: "..." } (traditional Workers AI), OpenAI-compatible
 * { choices: [{ message: { content } }] }, and plain strings.
 *
 * @param {any} response
 * @returns {string}
 */
export function extractText(response) {
  if (typeof response === "string") return response;
  if (!response || typeof response !== "object") return "";
  if (typeof response.response === "string") return response.response;
  const choice = response.choices?.[0];
  const content = choice?.message?.content;
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content.map((part) => (typeof part === "string" ? part : (part?.text ?? ""))).join("");
  }
  return "";
}

/**
 * Extract the first `{...}` JSON object from a text blob. Tolerates
 * leading/trailing prose, backtick fences, explanations — even though the
 * system prompt forbids them.
 *
 * @param {string} text
 * @returns {object | null}
 */
export function parseJudgementJson(text) {
  if (!text || typeof text !== "string") return null;
  // Strip code fences if the model disobeyed and wrapped the JSON.
  const unfenced = text.replace(/```(?:json)?\s*/gi, "").replace(/```/g, "");
  const start = unfenced.indexOf("{");
  if (start === -1) return null;
  // Walk brace depth to find the matching close.
  let depth = 0;
  let inString = false;
  let isEscaped = false;
  for (let i = start; i < unfenced.length; i++) {
    const ch = unfenced[i];
    if (inString) {
      if (isEscaped) isEscaped = false;
      else if (ch === "\\") isEscaped = true;
      else if (ch === '"') inString = false;
      continue;
    }
    if (ch === '"') inString = true;
    else if (ch === "{") depth++;
    else if (ch === "}") {
      depth--;
      if (depth === 0) {
        const slice = unfenced.slice(start, i + 1);
        try {
          return JSON.parse(slice);
        } catch {
          return null;
        }
      }
    }
  }
  return null;
}

/**
 * Coerce any parsed payload into the canonical shape with defaults.
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
 * Strip any case-insensitive whole-word occurrence of the secret from the
 * hint. Defense-in-depth — the system prompt forbids it but we don't trust
 * the model.
 *
 * @param {string} hint
 * @param {string} target
 * @returns {string}
 */
export function redactSecret(hint, target) {
  if (!target) return hint;
  const escaped = target.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const re = new RegExp(`\\b${escaped}\\b`, "ig");
  return hint.replace(re, "(redacted)");
}

/**
 * Judge a single user turn. Throws UpstreamError on AI failure.
 *
 * @param {{ AI: { run: (model: string, body: object) => Promise<any> } }} env
 * @param {import("./state.js").TwentyqGameState} state
 * @param {string} userInput — validated/normalized
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
    response = await env.AI.run(MODEL_ID, { messages });
  } catch (err) {
    console.log(
      JSON.stringify({
        msg: "twentyq_ai_throw",
        model: MODEL_ID,
        err: err instanceof Error ? { name: err.name, message: err.message } : String(err),
      }),
    );
    throw new UpstreamError("env.AI.run threw", { cause: err });
  }

  const text = extractText(response);
  const payload = parseJudgementJson(text);
  if (!payload) {
    console.log(
      JSON.stringify({
        msg: "twentyq_ai_unparseable",
        model: MODEL_ID,
        stage: "judge",
        preview: text.slice(0, 200),
      }),
    );
  }
  const judgement = normalizeJudgement(payload);
  judgement.hint = redactSecret(judgement.hint, state.target);
  return judgement;
}

/**
 * Generate { category, initialHint } for a fresh round's target keyword.
 * Uses the same JSON-in-content approach as judge(). On parse failure,
 * returns a safe generic fallback so the round can still start.
 *
 * @param {{ AI: { run: (model: string, body: object) => Promise<any> } }} env
 * @param {string} target
 * @returns {Promise<{ category: string, initialHint: string }>}
 */
export async function generateRoundStart(env, target) {
  if (!env?.AI?.run) {
    throw new UpstreamError("Workers AI binding not available");
  }
  const messages = [
    { role: "system", content: buildStartRoundPrompt(target) },
    { role: "user", content: "begin" },
  ];
  let response;
  try {
    response = await env.AI.run(MODEL_ID, { messages });
  } catch (err) {
    console.log(
      JSON.stringify({
        msg: "twentyq_ai_throw",
        model: MODEL_ID,
        stage: "roundstart",
        err: err instanceof Error ? { name: err.name, message: err.message } : String(err),
      }),
    );
    throw new UpstreamError("env.AI.run threw (roundstart)", { cause: err });
  }

  const text = extractText(response);
  const payload = parseJudgementJson(text);
  if (!payload || typeof payload.category !== "string" || typeof payload.initialHint !== "string") {
    console.log(
      JSON.stringify({
        msg: "twentyq_ai_unparseable",
        model: MODEL_ID,
        stage: "roundstart",
        preview: text.slice(0, 200),
      }),
    );
    return {
      category: "object",
      initialHint: "it is something you might encounter in everyday life",
    };
  }
  const category = payload.category.trim() || "object";
  const rawHint =
    payload.initialHint.trim() || "it is something you might encounter in everyday life";
  return {
    category,
    initialHint: redactSecret(rawHint, target),
  };
}
