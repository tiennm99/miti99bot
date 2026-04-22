/**
 * @file ConceptNet API client for the doantu module (Vietnamese).
 *
 * Mirrors semantle/api-client.js one-for-one — same endpoints, same
 * response shape — with two Vietnamese-specific changes:
 *   1. Concept URIs use `/c/vi/…` instead of `/c/en/…`.
 *   2. Multi-word terms are joined with an underscore for URL building
 *      (`con chó` → `/c/vi/con_chó`), so the board can still display the
 *      space-separated form while ConceptNet resolves the canonical URI.
 */

import { randomLine } from "./wordlist.js";

const DEFAULT_API_BASE = "https://api.conceptnet.io";
const DEFAULT_TIMEOUT_MS = 5000;
const USER_AGENT = "miti99bot/doantu";
const MAX_RANDOM_ATTEMPTS = 5;
const LANG = "vi";

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

/** `con chó` → `con_chó` — ConceptNet's concept-URI convention for phrases. */
function toConceptTerm(word) {
  return String(word).trim().replace(/\s+/g, "_");
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
 * @param {string} [apiBase] — override for mirrors/tests.
 * @param {{ timeoutMs?: number }} [opts]
 */
export function createClient(apiBase = DEFAULT_API_BASE, { timeoutMs = DEFAULT_TIMEOUT_MS } = {}) {
  /** @param {string} term */
  function concept(term) {
    const cn = toConceptTerm(term);
    return fetchJson(buildUrl(apiBase, `/c/${LANG}/${encodeURIComponent(cn)}`), timeoutMs);
  }

  /** @param {string} a @param {string} b */
  function relatedness(a, b) {
    return fetchJson(
      buildUrl(apiBase, "/relatedness", {
        node1: `/c/${LANG}/${toConceptTerm(a)}`,
        node2: `/c/${LANG}/${toConceptTerm(b)}`,
      }),
      timeoutMs,
    );
  }

  return {
    concept,
    relatedness,

    /**
     * Pick a target word from the local pool, verify via the concept endpoint,
     * fall back to an unverified pick after a few misses.
     * @returns {Promise<{ word: string, verified: boolean }>}
     */
    async randomWord() {
      for (let i = 0; i < MAX_RANDOM_ATTEMPTS; i++) {
        const candidate = randomLine();
        try {
          const c = await concept(candidate);
          if (hasEdges(c)) return { word: candidate, verified: true };
        } catch {
          // swallow — try the next candidate
        }
      }
      return { word: randomLine(), verified: false };
    },

    /**
     * Cosine-like similarity. Runs concept edge-check for `b` in parallel
     * with the relatedness call so OOV guesses are caught on the same round-trip.
     * Shape mirrors the semantle sibling.
     * @param {string} a
     * @param {string} b
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
        in_vocab_a: true,
        in_vocab_b: inVocabB,
        similarity: inVocabB ? value : null,
      };
    },
  };
}
