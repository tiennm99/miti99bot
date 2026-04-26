/**
 * @file mongo-client — memoized MongoDB Atlas client singleton.
 *
 * Exports `getDb(env)` for all Mongo-backed stores. On the first call from a
 * cold isolate it opens one connection; subsequent calls reuse the same
 * `MongoClient`. If `client.connect()` rejects, both `client` and
 * `connectPromise` are nulled so the next request retries cleanly instead of
 * reusing a dead client reference.
 *
 * `MongoServerSelectionError` (e.g. paused M0 cluster) is caught, logged with
 * an actionable message, then rethrown so the caller can map it to 503.
 *
 * @module db/mongo-client
 */

import { MongoClient } from "mongodb";

/** @type {MongoClient|null} */
let client = null;

/** @type {Promise<void>|null} */
let connectPromise = null;

/**
 * Return the memoized Db instance, connecting on the first call.
 *
 * Connection options match the Cloudflare Workers constraints:
 *   maxPoolSize: 1   — one connection per isolate
 *   minPoolSize: 0   — no idle keepalive (Workers tear down quickly)
 *   serverSelectionTimeoutMS: 5000 — fast fail for paused M0
 *   connectTimeoutMS: 10000        — TLS + SCRAM on cold start
 *
 * @param {{ MONGODB_URI: string }} env — Cloudflare Worker env (or test double).
 * @returns {Promise<import("mongodb").Db>}
 * @throws {import("mongodb").MongoServerSelectionError} if cluster unreachable.
 */
export async function getDb(env) {
  if (client) return client.db("miti99bot");

  if (!connectPromise) {
    client = new MongoClient(env.MONGODB_URI, {
      maxPoolSize: 1,
      minPoolSize: 0,
      serverSelectionTimeoutMS: 5000,
      connectTimeoutMS: 10000,
    });

    // On rejection: null BOTH so the next getDb() call retries with a fresh
    // client instead of awaiting the already-rejected promise (code-reviewer #16).
    connectPromise = client.connect().catch((err) => {
      client = null;
      connectPromise = null;
      throw err;
    });
  }

  try {
    await connectPromise;
  } catch (err) {
    // M0 clusters auto-pause after 60 days of inactivity. Surface an
    // actionable note so ops can identify and resume the cluster.
    // NOTE: URI is deliberately omitted to prevent credential leaks.
    if (err?.name === "MongoServerSelectionError") {
      console.warn(
        JSON.stringify({
          event: "mongo_server_selection_failed",
          note: "M0 may be paused — resume the cluster in Atlas, then retry. Caller should map to 503.",
        }),
      );
    }
    throw err;
  }

  return client.db("miti99bot");
}

/**
 * Close the active MongoClient and reset module-scope state.
 * Intended for test teardown and graceful shutdown only — not for
 * use in production request handlers.
 *
 * @returns {Promise<void>}
 */
export async function closeMongo() {
  if (client) {
    await client.close();
    client = null;
  }
  connectPromise = null;
}
