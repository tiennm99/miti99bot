/**
 * @file ConceptNet API client for the semantle module.
 *
 * ConceptNet endpoints:
 *   GET /relatedness?node1=/c/en/X&node2=/c/en/Y  → { value: number ∈ [-1, 1] }
 *   GET /c/en/{term}                              → { edges: [...] }   (empty ⇒ OOV)
 *
 * There is no official random-word endpoint, so the client picks a candidate
 * from our local `TARGET_POOL` and verifies it has a ConceptNet entry with
 * edges. After a few failed attempts it falls back to an unverified pick —
 * the curated pool is trusted enough that this should be rare.
 *
 * The returned `similarity(a, b)` shape mirrors the earlier word2sim contract
 * so handlers/render/state don't have to change.
 */

import { pickFromPool } from "./wordlist.js";

const DEFAULT_API_BASE = "https://api.conceptnet.io";
const DEFAULT_TIMEOUT_MS = 5000;
const USER_AGENT = "miti99bot/semantle";
const MAX_RANDOM_ATTEMPTS = 5;

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

function buildUrl(base, path, params = {}) {
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
    throw new UpstreamError("conceptnet fetch failed", { cause: err });
  }
  clearTimeout(timer);
  const text = await res.text();
  if (!res.ok) {
    throw new UpstreamError(`conceptnet HTTP ${res.status}`, {
      status: res.status,
      body: text.slice(0, 500),
    });
  }
  try {
    return JSON.parse(text);
  } catch (err) {
    throw new UpstreamError("conceptnet non-JSON response", { cause: err });
  }
}

function hasEdges(concept) {
  return Array.isArray(concept?.edges) && concept.edges.length > 0;
}

/**
 * @param {string} [apiBase] — override for mirrors/tests (default api.conceptnet.io)
 * @param {{ timeoutMs?: number }} [opts]
 */
export function createClient(apiBase = DEFAULT_API_BASE, { timeoutMs = DEFAULT_TIMEOUT_MS } = {}) {
  /** @param {string} term */
  function concept(term) {
    return fetchJson(buildUrl(apiBase, `/c/en/${encodeURIComponent(term)}`), timeoutMs);
  }

  /** @param {string} a @param {string} b */
  function relatedness(a, b) {
    return fetchJson(
      buildUrl(apiBase, "/relatedness", {
        node1: `/c/en/${a}`,
        node2: `/c/en/${b}`,
      }),
      timeoutMs,
    );
  }

  return {
    concept,
    relatedness,

    /**
     * Pick a target word from the local pool. Verifies each candidate has a
     * ConceptNet entry; falls back to an unverified pick after a few tries.
     * Shape matches the old word2sim `/random` response for handler reuse.
     * @returns {Promise<{ word: string, verified: boolean }>}
     */
    async randomWord() {
      for (let i = 0; i < MAX_RANDOM_ATTEMPTS; i++) {
        const candidate = pickFromPool();
        try {
          const c = await concept(candidate);
          if (hasEdges(c)) return { word: candidate, verified: true };
        } catch {
          // swallow — try the next candidate
        }
      }
      return { word: pickFromPool(), verified: false };
    },

    /**
     * Cosine-like similarity between `a` (target) and `b` (guess). Runs the
     * edge-check for `b` in parallel with the relatedness call so OOV guesses
     * are identified on the same round-trip.
     *
     * Shape deliberately mirrors the old word2sim response.
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
      const [conceptB, rel] = await Promise.all([concept(b), relatedness(a, b)]);
      const inVocabB = hasEdges(conceptB);
      const value = typeof rel?.value === "number" ? rel.value : null;
      return {
        a,
        b,
        canonical_a: a,
        canonical_b: b,
        in_vocab_a: true, // target was verified at round start
        in_vocab_b: inVocabB,
        similarity: inVocabB ? value : null,
      };
    },
  };
}

// Backwards-compat alias — older imports referenced `Word2SimError`.
export { UpstreamError as Word2SimError };
