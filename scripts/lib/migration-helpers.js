#!/usr/bin/env node
/**
 * @file migration-helpers — shared utilities for Phase 05 backfill scripts.
 *
 * Exports:
 *  - sleep, sha256
 *  - loadCheckpoint, saveCheckpoint, clearCheckpoint
 *  - cfKvList, cfKvGet
 *  - getMongoClient, closeMongoClient
 *  - countDiffRatio, sampleStrategy
 */

import { createHash } from "node:crypto";
import { existsSync, readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { MongoClient } from "mongodb";

const CF_API_BASE = "https://api.cloudflare.com/client/v4";

// ─── Primitives ───────────────────────────────────────────────────────────────

/** @param {number} ms @returns {Promise<void>} */
export const sleep = (ms) => new Promise((res) => setTimeout(res, ms));

/** @param {string} text @returns {string} hex SHA-256 */
export const sha256 = (text) => createHash("sha256").update(text, "utf8").digest("hex");

// ─── Checkpoint ───────────────────────────────────────────────────────────────

const cpPath = (mod) => resolve(process.cwd(), `.backfill-cursor-${mod}.json`);

/**
 * Load saved cursor state for a module. Returns null if none.
 *
 * @param {string} moduleName
 * @returns {{ cursor: string|null, lastKey: string|null }|null}
 */
export function loadCheckpoint(moduleName) {
  const p = cpPath(moduleName);
  if (!existsSync(p)) return null;
  try {
    return JSON.parse(readFileSync(p, "utf8"));
  } catch {
    return null;
  }
}

/**
 * Persist cursor state so a crash can resume from this page.
 *
 * @param {string} moduleName
 * @param {{ cursor: string|null, lastKey: string|null }} state
 */
export function saveCheckpoint(moduleName, state) {
  writeFileSync(cpPath(moduleName), JSON.stringify(state), "utf8");
}

/**
 * Remove checkpoint file on successful module completion.
 *
 * @param {string} moduleName
 */
export function clearCheckpoint(moduleName) {
  const p = cpPath(moduleName);
  if (existsSync(p)) {
    try {
      unlinkSync(p);
    } catch {
      // Non-fatal — may already be gone.
    }
  }
}

// ─── CF KV REST ───────────────────────────────────────────────────────────────

/**
 * List one page of KV keys for a given prefix.
 *
 * @param {string} accountId
 * @param {string} nsId
 * @param {string} token
 * @param {string} prefix
 * @param {string|null} cursor
 * @returns {Promise<{
 *   keys: Array<{name: string, metadata?: {expiration?: number}}>,
 *   cursor: string|null,
 *   list_complete: boolean
 * }>}
 */
export async function cfKvList(accountId, nsId, token, prefix, cursor) {
  const url = new URL(`${CF_API_BASE}/accounts/${accountId}/storage/kv/namespaces/${nsId}/keys`);
  url.searchParams.set("prefix", prefix);
  url.searchParams.set("limit", "1000");
  if (cursor) url.searchParams.set("cursor", cursor);

  const res = await fetch(url.toString(), {
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`CF KV list ${res.status}: ${body}`);
  }
  const json = await res.json();
  if (!json.success) throw new Error(`CF KV list error: ${JSON.stringify(json.errors ?? json)}`);
  return {
    keys: json.result ?? [],
    cursor: json.result_info?.cursor ?? null,
    list_complete: !json.result_info?.cursor,
  };
}

/**
 * Fetch the string value for a single KV key.
 *
 * @param {string} accountId
 * @param {string} nsId
 * @param {string} token
 * @param {string} key
 * @returns {Promise<string>}
 */
export async function cfKvGet(accountId, nsId, token, key) {
  const url = `${CF_API_BASE}/accounts/${accountId}/storage/kv/namespaces/${nsId}/values/${encodeURIComponent(key)}`;
  const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`CF KV get "${key}" ${res.status}: ${body}`);
  }
  return res.text();
}

/**
 * Write a string value into CF KV via the REST API.
 *
 * CF KV REST PUT: PUT /accounts/{id}/storage/kv/namespaces/{nsid}/values/{key}
 * Optional query param `expiration_ttl` (seconds from now, minimum 60).
 *
 * @param {string} accountId
 * @param {string} nsId
 * @param {string} token
 * @param {string} key
 * @param {string} value
 * @param {{ expirationTtl?: number }} [opts]
 * @returns {Promise<void>}
 */
export async function cfKvPut(accountId, nsId, token, key, value, opts = {}) {
  const url = new URL(
    `${CF_API_BASE}/accounts/${accountId}/storage/kv/namespaces/${nsId}/values/${encodeURIComponent(key)}`,
  );
  if (opts.expirationTtl != null) {
    url.searchParams.set("expiration_ttl", String(opts.expirationTtl));
  }
  const res = await fetch(url.toString(), {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "text/plain",
    },
    body: value,
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`CF KV put "${key}" ${res.status}: ${body}`);
  }
}

// ─── MongoDB singleton ────────────────────────────────────────────────────────

/** @type {MongoClient|null} */
let _client = null;

/**
 * Return a shared MongoClient, connecting on first call.
 * Registers process exit / signal handlers to close cleanly.
 *
 * @param {string} uri
 * @returns {Promise<MongoClient>}
 */
export async function getMongoClient(uri) {
  if (!uri) throw new Error("MongoDB URI is required — set it in .env.deploy and re-run");
  if (_client) return _client;
  const client = new MongoClient(uri);
  await client.connect();
  _client = client;
  const close = () => {
    _client?.close().catch(() => {});
    _client = null;
  };
  process.once("exit", close);
  process.once("SIGINT", () => {
    close();
    process.exit(0);
  });
  process.once("SIGTERM", () => {
    close();
    process.exit(0);
  });
  return _client;
}

/** Close the shared client (test teardown / forced close). */
export async function closeMongoClient() {
  if (_client) {
    await _client.close();
    _client = null;
  }
}

// ─── Parity math ─────────────────────────────────────────────────────────────

/**
 * Relative difference between two counts — 0 means identical.
 *
 * @param {number} a
 * @param {number} b
 * @returns {number} value in [0, 1]
 */
export const countDiffRatio = (a, b) => Math.abs(a - b) / Math.max(a, b, 1);

/**
 * Determine verify strategy: full-scan when total < 10 000,
 * otherwise random-sample N = min(500, ceil(sqrt(total))).
 *
 * @param {number} total
 * @returns {{ fullScan: boolean, sampleSize: number }}
 */
export function sampleStrategy(total) {
  if (total < 10000) return { fullScan: true, sampleSize: total };
  return { fullScan: false, sampleSize: Math.min(500, Math.ceil(Math.sqrt(total))) };
}
