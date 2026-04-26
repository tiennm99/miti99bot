/**
 * @file dual-kv-store — KVStore wrapper that writes to two backends in parallel.
 *
 * During the dual-write window (Phase 04 → Phase 07):
 *   - Reads go to the primary only (never the secondary).
 *   - Writes go to BOTH via `Promise.allSettled`. If the secondary fails,
 *     the failure is logged (key + error class + message only — no document value
 *     to avoid PII leakage) and enqueued to a KV retry queue. The primary
 *     failure causes a throw; secondary failure is transparent to the caller.
 *
 * Retry queue keys: `__retry:mongo-failed:<random-id>` stored in `env.KV`
 * (the raw CF KV namespace, not the dual-store itself).
 *
 * Expose `_kind = "dual"` sentinel for test-side identification.
 *
 * @module db/dual-kv-store
 */

/**
 * @typedef {import("./kv-store-interface.js").KVStore} KVStore
 * @typedef {import("./kv-store-interface.js").KVStorePutOptions} KVStorePutOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListOptions} KVStoreListOptions
 * @typedef {import("./kv-store-interface.js").KVStoreListResult} KVStoreListResult
 */

const RETRY_PREFIX = "__retry:mongo-failed:";

/**
 * Generate a short random suffix for retry-queue keys so concurrent failures
 * do not collide.
 *
 * @returns {string}
 */
function randomId() {
  return Math.random().toString(36).slice(2, 10);
}

/**
 * Enqueue a failed secondary write to the raw KV namespace so the
 * drift-verifier cron can retry it later.
 *
 * @param {any} rawKv — raw CF KVNamespace (env.KV)
 * @param {object} payload — { op, key, value?, opts? }
 * @returns {Promise<void>}
 */
async function enqueueRetry(rawKv, payload) {
  try {
    const id = `${RETRY_PREFIX}${payload.key}:${randomId()}`;
    await rawKv.put(id, JSON.stringify({ ...payload, ts: Date.now() }));
  } catch (err) {
    // Retry-queue write failing is unfortunate but must not crash the request.
    console.warn("[dual-kv] enqueueRetry failed", {
      phase: "dual-kv",
      op: "enqueue",
      err: err instanceof Error ? err.message : String(err),
    });
  }
}

/**
 * @implements {KVStore}
 */
export class DualKVStore {
  /**
   * @param {KVStore} primary — the authoritative store (reads + writes).
   * @param {KVStore} secondary — the mirror store (writes only; failures silently queued).
   * @param {any} rawKv — raw CF KVNamespace used for the retry queue.
   * @param {object} [logger] — injectable logger; defaults to `console`.
   */
  constructor(primary, secondary, rawKv, logger = console) {
    if (!primary) throw new Error("DualKVStore: primary is required");
    if (!secondary) throw new Error("DualKVStore: secondary is required");
    if (!rawKv) throw new Error("DualKVStore: rawKv is required");
    this._primary = primary;
    this._secondary = secondary;
    this._rawKv = rawKv;
    this._log = logger;
    /** @type {"dual"} */
    this._kind = "dual";
  }

  /**
   * Fire both writes in parallel. Throw on primary failure. On secondary
   * failure: log structured warning and enqueue for retry.
   *
   * @param {string} op — operation name for logging.
   * @param {string} key — the key being written (used in retry payload + logs).
   * @param {Function} primaryFn — async thunk for primary write.
   * @param {Function} secondaryFn — async thunk for secondary write.
   * @param {object} [retryPayload] — extra fields stored in the retry queue entry.
   * @returns {Promise<any>} primary result.
   */
  async _dualWrite(op, key, primaryFn, secondaryFn, retryPayload) {
    const [primaryResult, secondaryResult] = await Promise.allSettled([primaryFn(), secondaryFn()]);

    if (secondaryResult.status === "rejected") {
      const err = secondaryResult.reason;
      this._log.warn("[dual-kv] secondary write failed", {
        phase: "dual-kv",
        op,
        key,
        errClass: err instanceof Error ? err.constructor.name : "unknown",
        err: err instanceof Error ? err.message : String(err),
      });
      await enqueueRetry(this._rawKv, { op, key, ...retryPayload });
    }

    if (primaryResult.status === "rejected") {
      throw primaryResult.reason;
    }

    return primaryResult.value;
  }

  /**
   * @param {string} key
   * @returns {Promise<string|null>}
   */
  async get(key) {
    return this._primary.get(key);
  }

  /**
   * @param {string} key
   * @param {string} value
   * @param {KVStorePutOptions} [opts]
   * @returns {Promise<void>}
   */
  async put(key, value, opts) {
    return this._dualWrite(
      "put",
      key,
      () => this._primary.put(key, value, opts),
      () => this._secondary.put(key, value, opts),
      { opts },
    );
  }

  /**
   * @param {string} key
   * @returns {Promise<void>}
   */
  async delete(key) {
    return this._dualWrite(
      "delete",
      key,
      () => this._primary.delete(key),
      () => this._secondary.delete(key),
      {},
    );
  }

  /**
   * @param {KVStoreListOptions} [opts]
   * @returns {Promise<KVStoreListResult>}
   */
  async list(opts = {}) {
    return this._primary.list(opts);
  }

  /**
   * @param {string} key
   * @returns {Promise<any|null>}
   */
  async getJSON(key) {
    return this._primary.getJSON(key);
  }

  /**
   * @param {string} key
   * @param {any} value
   * @param {KVStorePutOptions} [opts]
   * @returns {Promise<void>}
   */
  async putJSON(key, value, opts) {
    return this._dualWrite(
      "putJSON",
      key,
      () => this._primary.putJSON(key, value, opts),
      () => this._secondary.putJSON(key, value, opts),
      { opts },
    );
  }
}
