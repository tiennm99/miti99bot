/**
 * @file fake-d1 — in-memory D1-like fake for unit tests.
 *
 * Supports a limited subset of SQL semantics needed by the test suite:
 *   - INSERT: appends a row built from binds.
 *   - SELECT: returns all rows for the matched table.
 *   - SELECT DISTINCT <col>: returns unique values for a single column.
 *   - SELECT id FROM <table> WHERE user_id = ? ORDER BY ts DESC: returns ids sorted by ts DESC.
 *   - SELECT id FROM <table> ORDER BY ts DESC: global sort by ts DESC.
 *   - DELETE WHERE id IN (?,...): removes specific rows by id.
 *   - DELETE (generic): clears entire table (fallback for legacy tests).
 *
 * Supported operations:
 *   prepare(query, ...binds) → fake prepared statement
 *     .run()   → { meta: { changes, last_row_id } }
 *     .all()   → { results: row[] }
 *     .first() → row | null
 *   batch(stmts) → array of { results: row[] }
 *
 * Usage in tests:
 *   const fakeDb = makeFakeD1();
 *   fakeDb.seed("mymod_foo", [{ id: 1, val: "x" }]);
 *   const sql = createSqlStore("mymod", { DB: fakeDb });
 *   const rows = await sql.all("SELECT * FROM mymod_foo");
 *   // rows === [{ id: 1, val: "x" }]
 */

export function makeFakeD1() {
  /** @type {Map<string, any[]>} table name → rows */
  const tables = new Map();

  /** @type {Array<{query: string, binds: any[]}>} */
  const runLog = [];

  /** @type {Array<{query: string, binds: any[]}>} */
  const queryLog = [];

  /**
   * Pre-populate a table with rows.
   *
   * @param {string} table
   * @param {any[]} rows
   */
  function seed(table, rows) {
    tables.set(table, [...rows]);
  }

  /**
   * Extract the first table name token from a query string.
   * Handles simple patterns: FROM <table>, INTO <table>, UPDATE <table>.
   *
   * @param {string} query
   * @returns {string|null}
   */
  function extractTable(query) {
    const normalized = query.replace(/\s+/g, " ").trim();
    const m =
      normalized.match(/\bFROM\s+(\w+)/i) ||
      normalized.match(/\bINTO\s+(\w+)/i) ||
      normalized.match(/\bUPDATE\s+(\w+)/i) ||
      normalized.match(/\bTABLE\s+(\w+)/i);
    return m ? m[1] : null;
  }

  /**
   * Parse DELETE WHERE id IN (...) — returns the set of ids to delete, or null
   * if the query doesn't match this pattern (fall back to clear-all).
   *
   * Matches patterns like:
   *   DELETE FROM t WHERE id IN (?,?,?)
   *   DELETE FROM t WHERE id IN (SELECT id FROM t ...)  — not supported, returns null
   *
   * @param {string} query
   * @param {any[]} binds
   * @returns {Set<any>|null}
   */
  function parseDeleteIds(query, binds) {
    const normalized = query.replace(/\s+/g, " ").trim();
    // Detect DELETE ... WHERE id IN (?,?,?) with only ? placeholders (no subquery).
    const m = normalized.match(/\bWHERE\s+id\s+IN\s*\(([^)]+)\)/i);
    if (!m) return null;

    const inner = m[1].trim();
    // If inner contains SELECT, it's a subquery — not supported in fake.
    if (/\bSELECT\b/i.test(inner)) return null;

    // Count placeholders and consume from binds.
    const placeholders = inner.split(",").map((s) => s.trim());
    if (placeholders.every((p) => p === "?")) {
      return new Set(binds.slice(0, placeholders.length));
    }
    return null;
  }

  /**
   * Resolve a raw token from a regex match: if it's "?" consume the next bind
   * value from the iterator; otherwise parse it as a number literal.
   *
   * @param {string} raw — captured token from regex (e.g. "?" or "-1" or "10")
   * @param {Iterator<any>} bindIter — iterator over remaining binds
   * @returns {number}
   */
  function resolveNumericToken(raw, bindIter) {
    if (raw === "?") {
      const next = bindIter.next();
      return next.done ? 0 : Number(next.value);
    }
    return Number(raw);
  }

  /**
   * Execute a SELECT query with limited semantic understanding.
   * Handles:
   *   - SELECT DISTINCT user_id FROM <table>
   *   - SELECT id FROM <table> WHERE user_id = ? ORDER BY ts DESC [LIMIT <n|?> [OFFSET <n|?>]]
   *   - SELECT id FROM <table> ORDER BY ts DESC [LIMIT <n|?> [OFFSET <n|?>]]
   *   - SELECT * / general → returns all rows
   *
   * LIMIT/OFFSET tokens may be numeric literals OR "?" bound parameters.
   *
   * @param {string} query
   * @param {any[]} binds
   * @returns {any[]}
   */
  function executeSelect(query, binds) {
    const normalized = query.replace(/\s+/g, " ").trim();
    const table = extractTable(normalized);
    const rows = table ? (tables.get(table) ?? []) : [];

    // SELECT DISTINCT user_id FROM <table>
    if (/SELECT\s+DISTINCT\s+user_id\b/i.test(normalized)) {
      const seen = new Set();
      const result = [];
      for (const row of rows) {
        if (!seen.has(row.user_id)) {
          seen.add(row.user_id);
          result.push({ user_id: row.user_id });
        }
      }
      return result;
    }

    // SELECT id FROM <table> WHERE user_id = ? ORDER BY ts DESC [LIMIT <n|?> [OFFSET <n|?>]]
    // Binds layout: [userId, ...optional LIMIT bind, ...optional OFFSET bind]
    const whereUserRe =
      /SELECT\s+id\s+FROM\s+\w+\s+WHERE\s+user_id\s*=\s*\?\s+ORDER\s+BY\s+ts\s+DESC(?:\s+LIMIT\s+(\S+)(?:\s+OFFSET\s+(\S+))?)?/i;
    const whereUserMatch = normalized.match(whereUserRe);
    if (whereUserMatch) {
      const userId = binds[0];
      let filtered = rows.filter((r) => r.user_id === userId);
      // Sort by ts DESC.
      filtered = [...filtered].sort((a, b) => b.ts - a.ts);

      // Binds after userId start at index 1.
      const remainingBinds = binds.slice(1)[Symbol.iterator]();
      const rawLimit = whereUserMatch[1];
      const rawOffset = whereUserMatch[2];

      let offset = 0;
      let limit;
      if (rawLimit !== undefined) {
        limit = resolveNumericToken(rawLimit, remainingBinds);
        if (rawOffset !== undefined) {
          offset = resolveNumericToken(rawOffset, remainingBinds);
        }
      }

      if (offset > 0) filtered = filtered.slice(offset);
      // Negative limit (e.g. -1) = all rows; skip slicing.
      if (limit !== undefined && limit >= 0) filtered = filtered.slice(0, limit);

      return filtered.map((r) => ({ id: r.id }));
    }

    // SELECT id FROM <table> ORDER BY ts DESC [LIMIT <n|?> [OFFSET <n|?>]]
    const globalOrderRe =
      /SELECT\s+id\s+FROM\s+\w+\s+ORDER\s+BY\s+ts\s+DESC(?:\s+LIMIT\s+(\S+)(?:\s+OFFSET\s+(\S+))?)?/i;
    const globalOrderMatch = normalized.match(globalOrderRe);
    if (globalOrderMatch) {
      let sorted = [...rows].sort((a, b) => b.ts - a.ts);

      const bindIter = binds[Symbol.iterator]();
      const rawLimit = globalOrderMatch[1];
      const rawOffset = globalOrderMatch[2];

      let offset = 0;
      let limit;
      if (rawLimit !== undefined) {
        limit = resolveNumericToken(rawLimit, bindIter);
        if (rawOffset !== undefined) {
          offset = resolveNumericToken(rawOffset, bindIter);
        }
      }

      if (offset > 0) sorted = sorted.slice(offset);
      // Negative limit (e.g. -1) = all rows; skip slicing.
      if (limit !== undefined && limit >= 0) sorted = sorted.slice(0, limit);

      return sorted.map((r) => ({ id: r.id }));
    }

    // Generic SELECT → return all rows.
    return rows;
  }

  /**
   * Build a fake prepared statement.
   *
   * @param {string} query
   * @param {any[]} binds
   */
  function makePrepared(query, binds) {
    return {
      bind(...moreBinds) {
        return makePrepared(query, [...binds, ...moreBinds]);
      },

      async run() {
        runLog.push({ query, binds });
        const table = extractTable(query);
        const upper = query.trim().toUpperCase();

        // Simulate INSERT: push a row built from binds.
        if (upper.startsWith("INSERT") && table) {
          const existing = tables.get(table) ?? [];
          const newRow = { _binds: binds };
          tables.set(table, [...existing, newRow]);
          return { meta: { changes: 1, last_row_id: existing.length + 1 } };
        }

        // DELETE: check for WHERE id IN (...) first, otherwise clear table.
        if (upper.startsWith("DELETE") && table) {
          const existing = tables.get(table) ?? [];
          const deleteIds = parseDeleteIds(query, binds);
          if (deleteIds !== null) {
            // Targeted delete by id set.
            const remaining = existing.filter((r) => !deleteIds.has(r.id));
            const changes = existing.length - remaining.length;
            tables.set(table, remaining);
            return { meta: { changes, last_row_id: 0 } };
          }
          // Fallback: clear all (naive — legacy tests rely on this).
          tables.set(table, []);
          return { meta: { changes: existing.length, last_row_id: 0 } };
        }

        return { meta: { changes: 0, last_row_id: 0 } };
      },

      async all() {
        queryLog.push({ query, binds });
        const results = executeSelect(query, binds);
        return { results };
      },

      async first() {
        queryLog.push({ query, binds });
        const results = executeSelect(query, binds);
        return results[0] ?? null;
      },
    };
  }

  return {
    tables,
    runLog,
    queryLog,
    seed,

    /**
     * D1Database.prepare() — returns a fake prepared statement.
     *
     * @param {string} query
     * @returns {ReturnType<typeof makePrepared>}
     */
    prepare(query) {
      return makePrepared(query, []);
    },

    /**
     * D1Database.batch() — runs each statement's all() and collects results.
     *
     * @param {Array<ReturnType<typeof makePrepared>>} statements
     * @returns {Promise<Array<{results: any[]}>>}
     */
    async batch(statements) {
      return Promise.all(statements.map((s) => s.all()));
    },
  };
}
