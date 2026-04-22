/**
 * @file Cloudflare Workers AI client for the doantu module (Vietnamese semantle).
 *
 * Mirrors semantle/api-client.js but uses `@cf/baai/bge-m3` — BAAI's
 * multilingual embedding model — because the English-only BGE variants
 * can't produce meaningful Vietnamese vectors (their tokenizer is
 * English-centric and Vietnamese diacritics get shredded into noisy
 * byte-level subwords).
 *
 * Vocabulary: the curated `words-data.js` list (duyet/vietnamese-wordlist
 * Viet22K) doubles as the in/out-of-vocabulary set. Lookups are O(1) via
 * Set.has(), so OOV detection needs no extra round-trip.
 *
 * The returned `similarity(a, b)` shape matches the semantle sibling so
 * handlers/render/state can be reused unchanged.
 */

import { randomLine } from "./wordlist.js";
import WORDS from "./words-data.js";

// BGE-M3: multilingual (194 languages incl. Vietnamese), 1024 dimensions,
// ~1,075 Neurons per M input tokens — cheaper than bge-small-en-v1.5.
const DEFAULT_MODEL = "@cf/baai/bge-m3";

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
     * Pick a target word from the local Vietnamese pool. The pool IS the
     * vocabulary, so every pick is trivially verified.
     * @returns {Promise<{ word: string, verified: boolean }>}
     */
    async randomWord() {
      return { word: randomLine(), verified: true };
    },

    /**
     * Cosine similarity between `a` (target) and `b` (guess). Uses the local
     * Vietnamese wordlist as vocabulary — unknown words return
     * `in_vocab_b: false` with `similarity: null` and skip inference.
     *
     * @param {string} a
     * @param {string} b
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
