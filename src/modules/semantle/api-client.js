/**
 * @file word2sim HTTP API client.
 *
 * Wraps two endpoints:
 *   GET /random      → pick a secret word at round start
 *   GET /similarity  → cosine similarity between target and guess per turn
 *
 * Stateless. No caching layer — word2sim itself is cheap enough, and caching
 * per-pair scores in KV would pollute the namespace without measurable gain.
 */

const DEFAULT_TIMEOUT_MS = 5000;
const USER_AGENT = "miti99bot/semantle";

export class Word2SimError extends Error {
  /** @param {string} message @param {{status?: number, body?: string, cause?: unknown}} [meta] */
  constructor(message, meta = {}) {
    super(message);
    this.name = "Word2SimError";
    this.status = meta.status;
    this.body = meta.body;
    if (meta.cause !== undefined) this.cause = meta.cause;
  }
}

function buildUrl(base, path, params) {
  const normalized = String(base).replace(/\/+$/, "");
  const url = new URL(`${normalized}${path}`);
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null) continue;
    url.searchParams.set(k, String(v));
  }
  return url.toString();
}

async function fetchJson(url, timeoutMs) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  let res;
  try {
    res = await fetch(url, {
      headers: { "User-Agent": USER_AGENT, Accept: "application/json" },
      signal: controller.signal,
    });
  } catch (err) {
    clearTimeout(timer);
    throw new Word2SimError("word2sim fetch failed", { cause: err });
  }
  clearTimeout(timer);
  const text = await res.text();
  if (!res.ok) {
    throw new Word2SimError(`word2sim HTTP ${res.status}`, {
      status: res.status,
      body: text.slice(0, 500),
    });
  }
  try {
    return JSON.parse(text);
  } catch (err) {
    throw new Word2SimError("word2sim non-JSON response", { cause: err });
  }
}

/**
 * @param {string} apiBase — e.g. "https://word2sim.sg.miti99.com"
 * @param {{ timeoutMs?: number }} [opts]
 */
export function createClient(apiBase, { timeoutMs = DEFAULT_TIMEOUT_MS } = {}) {
  return {
    /**
     * Pick a random vocab word matching filters.
     * @param {Record<string, string|number|boolean>} [filters]
     * @returns {Promise<{ word: string, rank: number }>}
     */
    randomWord(filters = {}) {
      return fetchJson(buildUrl(apiBase, "/random", filters), timeoutMs);
    },
    /**
     * Cosine similarity between two words.
     * @param {string} a
     * @param {string} b
     * @returns {Promise<{
     *   a: string, b: string,
     *   canonical_a: string|null, canonical_b: string|null,
     *   in_vocab_a: boolean, in_vocab_b: boolean,
     *   similarity: number|null
     * }>}
     */
    similarity(a, b) {
      return fetchJson(buildUrl(apiBase, "/similarity", { a, b }), timeoutMs);
    },
  };
}
