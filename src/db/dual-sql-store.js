/**
 * @file dual-sql-store — SqlStore wrapper that writes to two backends in parallel.
 *
 * Same fault-tolerance model as DualKVStore:
 *   - Reads (`all`, `first`) go to the primary only.
 *   - `run` writes go to BOTH via `Promise.allSettled`. Secondary failure is
 *     logged (query + error — no row values to avoid PII) and enqueued to the
 *     KV retry queue. Primary failure causes a throw.
 *   - `prepare` and `batch` go to primary only — D1 prepared statements are
 *     CF-specific and cannot be forwarded to MongoDB.
 *
 * Retry queue keys: `__retry:mongo-sql-failed:<random-id>` stored in `env.KV`.
 *
 * Expose `_kind = "dual"` sentinel for test-side identification.
 *
 * @module db/dual-sql-store
 */

/**
 * @typedef {import("./sql-store-interface.js").SqlStore} SqlStore
 * @typedef {import("./sql-store-interface.js").SqlRunResult} SqlRunResult
 */

const RETRY_PREFIX = "__retry:mongo-sql-failed:";

/**
 * Generate a short random suffix for retry-queue keys.
 *
 * @returns {string}
 */
function randomId() {
  return Math.random().toString(36).slice(2, 10);
}

/**
 * Enqueue a failed secondary write to the raw KV namespace.
 *
 * @param {any} rawKv — raw CF KVNamespace (env.KV)
 * @param {object} payload — { op, query, binds? }
 * @returns {Promise<void>}
 */
async function enqueueRetry(rawKv, payload) {
  try {
    const slug = payload.query.slice(0, 40).replace(/\s+/g, "_");
    const id = `${RETRY_PREFIX}${slug}:${randomId()}`;
    await rawKv.put(id, JSON.stringify({ ...payload, ts: Date.now() }));
  } catch (err) {
    console.warn("[dual-sql] enqueueRetry failed", {
      phase: "dual-sql",
      op: "enqueue",
      err: err instanceof Error ? err.message : String(err),
    });
  }
}

/**
 * @implements {SqlStore}
 */
export class DualSqlStore {
  /**
   * @param {SqlStore} primary — the authoritative store (reads + writes).
   * @param {SqlStore} secondary — the mirror store (writes only; failures silently queued).
   * @param {any} rawKv — raw CF KVNamespace used for the retry queue.
   * @param {object} [logger] — injectable logger; defaults to `console`.
   */
  constructor(primary, secondary, rawKv, logger = console) {
    if (!primary) throw new Error("DualSqlStore: primary is required");
    if (!secondary) throw new Error("DualSqlStore: secondary is required");
    if (!rawKv) throw new Error("DualSqlStore: rawKv is required");
    this._primary = primary;
    this._secondary = secondary;
    this._rawKv = rawKv;
    this._log = logger;
    /** @type {"dual"} */
    this._kind = "dual";
    /** @type {string} */
    this.tablePrefix = primary.tablePrefix ?? "";
  }

  /**
   * Execute a write statement against both stores. Throw on primary failure;
   * log + enqueue on secondary failure.
   *
   * @param {string} query
   * @param {any[]} binds
   * @returns {Promise<SqlRunResult>}
   */
  async run(query, ...binds) {
    const [primaryResult, secondaryResult] = await Promise.allSettled([
      this._primary.run(query, ...binds),
      this._secondary.run(query, ...binds),
    ]);

    if (secondaryResult.status === "rejected") {
      const err = secondaryResult.reason;
      this._log.warn("[dual-sql] secondary run failed", {
        phase: "dual-sql",
        op: "run",
        // Log query shape only; never log bind values (PII risk).
        query: query.slice(0, 80),
        errClass: err instanceof Error ? err.constructor.name : "unknown",
        err: err instanceof Error ? err.message : String(err),
      });
      await enqueueRetry(this._rawKv, { op: "run", query, binds });
    }

    if (primaryResult.status === "rejected") {
      throw primaryResult.reason;
    }

    return primaryResult.value;
  }

  /**
   * Execute a SELECT and return all matching rows — primary only.
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {Promise<any[]>}
   */
  async all(query, ...binds) {
    return this._primary.all(query, ...binds);
  }

  /**
   * Execute a SELECT and return the first row — primary only.
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {Promise<any|null>}
   */
  async first(query, ...binds) {
    return this._primary.first(query, ...binds);
  }

  /**
   * Returns a prepared statement from the primary store only.
   * CF-specific; cannot be forwarded to MongoDB.
   *
   * @param {string} query
   * @param {...any} binds
   * @returns {any}
   */
  prepare(query, ...binds) {
    return this._primary.prepare(query, ...binds);
  }

  /**
   * Execute multiple prepared statements — primary only.
   *
   * @param {any[]} statements
   * @returns {Promise<any[]>}
   */
  async batch(statements) {
    return this._primary.batch(statements);
  }
}
