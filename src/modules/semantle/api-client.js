/**
 * @file Cloudflare Workers AI client for the semantle module.
 *
 * Runs the `@cf/baai/bge-small-en-v1.5` text-embedding model via the `env.AI`
 * binding, then scores guesses by computing cosine similarity between the
 * target and guess vectors locally (no extra round-trip).
 *
 * Vocabulary: the curated `words-data.js` list (google-10k) doubles as our
 * in/out-of-vocabulary set — anything outside it is treated as OOV so players
 * get the "not in the vocabulary" reply instead of a noisy embedding score.
 */

import { randomLine } from "./wordlist.js";
import WORDS from "./words-data.js";

const DEFAULT_MODEL = "@cf/baai/bge-small-en-v1.5";

// O(1) membership lookup for OOV detection. Built once per isolate.
const VOCAB = new Set(WORDS);

export class UpstreamError extends Error {
  /** @param {string} message @param {{status?: number, body?: string, cause?: unknown}} [meta] */
  constructor(message, meta = {}) {
    super(message);
    this.name = "UpstreamError";
    this.status = meta.status;
    this.body = meta.body;
    if (meta.cause !== undefined) this.cause = meta.cause;
  }
}

function cosineSimilarity(a, b) {
  if (!a || !b || a.length !== b.length) return null;
  let dot = 0;
  let normA = 0;
  let normB = 0;
  for (let i = 0; i < a.length; i++) {
    dot += a[i] * b[i];
    normA += a[i] * a[i];
    normB += b[i] * b[i];
  }
  const denom = Math.sqrt(normA) * Math.sqrt(normB);
  return denom === 0 ? null : dot / denom;
}

/**
 * @param {{ run: (model: string, input: { text: string[] }) => Promise<{ data: number[][] }> }} ai
 *   — Workers AI binding (`env.AI`). Tests pass a fake with the same `.run()` shape.
 * @param {{ model?: string }} [opts]
 */
export function createClient(ai, { model = DEFAULT_MODEL } = {}) {
  if (!ai || typeof ai.run !== "function") {
    throw new TypeError("createClient: ai binding with .run(model, input) is required");
  }

  async function embedPair(a, b) {
    let resp;
    try {
      resp = await ai.run(model, { text: [a, b] });
    } catch (err) {
      throw new UpstreamError("workers-ai embedding failed", { cause: err });
    }
    const data = resp?.data;
    if (!Array.isArray(data) || data.length < 2) {
      throw new UpstreamError("workers-ai returned malformed embedding payload");
    }
    return [data[0], data[1]];
  }

  return {
    /**
     * Pick a target word from the local pool. The pool IS our vocabulary,
     * so every pick is trivially verified — no upstream check needed.
     * @returns {Promise<{ word: string, verified: boolean }>}
     */
    async randomWord() {
      return { word: randomLine(), verified: true };
    },

    /**
     * Cosine similarity between `a` (target) and `b` (guess). Uses the local
     * wordlist as the vocabulary — unknown words return `in_vocab_b: false`
     * with `similarity: null` and skip the inference call entirely.
     *
     * @param {string} a
     * @param {string} b
     * @returns {Promise<{
     *   a: string, b: string,
     *   canonical_a: string, canonical_b: string,
     *   in_vocab_a: boolean, in_vocab_b: boolean,
     *   similarity: number | null,
     * }>}
     */
    async similarity(a, b) {
      const base = { a, b, canonical_a: a, canonical_b: b, in_vocab_a: true };
      if (!VOCAB.has(b)) {
        return { ...base, in_vocab_b: false, similarity: null };
      }
      const [vecA, vecB] = await embedPair(a, b);
      const sim = cosineSimilarity(vecA, vecB);
      return { ...base, in_vocab_b: true, similarity: sim };
    },
  };
}
