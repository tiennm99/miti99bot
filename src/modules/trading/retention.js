/**
 * @file retention — daily cron handler that trims trading_trades to enforce row caps.
 *
 * Strategy (two-pass):
 *   1. Per-user pass: for each distinct user_id, delete rows beyond PER_USER_CAP
 *      (keeps the newest N rows per user).
 *   2. Global FIFO pass: delete any rows beyond GLOBAL_CAP across all users
 *      (keeps the newest N rows globally).
 *
 * Uses a hybrid SELECT-then-DELETE approach so it works with both the real
 * Cloudflare D1 binding and the in-memory fake-d1 used in unit tests.
 *
 * @typedef {import("../../db/sql-store-interface.js").SqlStore} SqlStore
 */

const TABLE = "trading_trades";

/** Default per-user row cap. Exported for testability. */
export const PER_USER_CAP = 1000;

/** Default global row cap across all users. Exported for testability. */
export const GLOBAL_CAP = 10000;

/**
 * Build a dynamic `DELETE FROM <table> WHERE id IN (?, ?, ...)` query.
 * Returns [query, ids] tuple — ids passed as spread binds.
 *
 * @param {number[]} ids
 * @returns {[string, number[]]}
 */
function buildDeleteByIds(ids) {
  const placeholders = ids.map(() => "?").join(", ");
  return [`DELETE FROM ${TABLE} WHERE id IN (${placeholders})`, ids];
}

/**
 * Daily cron handler — trims trading_trades to enforce per-user and global caps.
 *
 * @param {any} _event — Cloudflare ScheduledEvent (unused; present for handler contract).
 * @param {{ sql: SqlStore | null }} ctx
 * @param {{ perUserCap?: number, globalCap?: number }} [caps] — override caps (for tests).
 * @returns {Promise<void>}
 */
export async function trimTradesHandler(_event, { sql }, caps = {}) {
  if (sql === null) {
    console.log("[trim-trades] no D1 binding — skipping");
    return;
  }

  const perUserCap = caps.perUserCap ?? PER_USER_CAP;
  const globalCap = caps.globalCap ?? GLOBAL_CAP;

  let perUserDeleted = 0;

  // ── Pass 1: per-user trim ──────────────────────────────────────────────────
  const userRows = await sql.all(`SELECT DISTINCT user_id FROM ${TABLE}`);

  for (const row of userRows) {
    const userId = row.user_id;

    // Fetch IDs of rows that exceed the per-user cap (oldest rows = beyond OFFSET perUserCap).
    const excessRows = await sql.all(
      `SELECT id FROM ${TABLE} WHERE user_id = ? ORDER BY ts DESC LIMIT -1 OFFSET ?`,
      userId,
      perUserCap,
    );

    if (excessRows.length === 0) continue;

    const ids = excessRows.map((r) => r.id);
    const [query, binds] = buildDeleteByIds(ids);
    const result = await sql.run(query, ...binds);
    perUserDeleted += result.changes ?? ids.length;
  }

  console.log(`[trim-trades] per-user pass: deleted ${perUserDeleted} rows`);

  // ── Pass 2: global FIFO trim ───────────────────────────────────────────────
  const globalExcess = await sql.all(
    `SELECT id FROM ${TABLE} ORDER BY ts DESC LIMIT -1 OFFSET ?`,
    globalCap,
  );

  let globalDeleted = 0;
  if (globalExcess.length > 0) {
    const ids = globalExcess.map((r) => r.id);
    const [query, binds] = buildDeleteByIds(ids);
    const result = await sql.run(query, ...binds);
    globalDeleted = result.changes ?? ids.length;
  }

  console.log(`[trim-trades] global pass: deleted ${globalDeleted} rows`);
  console.log(`[trim-trades] total deleted: ${perUserDeleted + globalDeleted} rows`);
}
