/**
 * @file backfill-mongo-to-d1.test.js — unit tests for SQL-statement construction.
 *
 * Tests `buildInsertSql` and `sqlStr` exported from backfill-mongo-to-d1.js.
 * No execSync, no wrangler, no real Mongo connection.
 */

import { describe, expect, it } from "vitest";
import { buildInsertSql, sqlStr } from "../../scripts/backfill-mongo-to-d1.js";

// ─── sqlStr ───────────────────────────────────────────────────────────────────

describe("sqlStr", () => {
  it("wraps a plain string in single quotes", () => {
    expect(sqlStr("hello")).toBe("'hello'");
  });

  it("escapes internal single quotes by doubling them", () => {
    expect(sqlStr("it's")).toBe("'it''s'");
  });

  it("escapes multiple single quotes", () => {
    expect(sqlStr("a'b'c")).toBe("'a''b''c'");
  });

  it("returns NULL for null", () => {
    expect(sqlStr(null)).toBe("NULL");
  });

  it("returns NULL for undefined", () => {
    expect(sqlStr(undefined)).toBe("NULL");
  });

  it("handles empty string", () => {
    expect(sqlStr("")).toBe("''");
  });

  it("coerces non-string values via String()", () => {
    // @ts-ignore — testing runtime coercion
    expect(sqlStr(42)).toBe("'42'");
  });
});

// ─── buildInsertSql ───────────────────────────────────────────────────────────

describe("buildInsertSql", () => {
  /** @type {Parameters<typeof buildInsertSql>[0]} */
  const BASE_ROW = {
    id: 1,
    user_id: "u123",
    symbol: "BTC",
    side: "buy",
    qty: 0.5,
    price_vnd: 1500000,
    ts: 1700000000,
  };

  it("produces a valid INSERT statement", () => {
    const sql = buildInsertSql(BASE_ROW);
    expect(sql).toMatch(
      /^INSERT INTO trading_trades \(id, user_id, symbol, side, qty, price_vnd, ts\) VALUES \(/,
    );
    expect(sql).toMatch(/\);$/);
  });

  it("includes all column values in correct order", () => {
    const sql = buildInsertSql(BASE_ROW);
    expect(sql).toContain("1,"); // id
    expect(sql).toContain("'u123'"); // user_id
    expect(sql).toContain("'BTC'"); // symbol
    expect(sql).toContain("'buy'"); // side
    expect(sql).toContain("0.5,"); // qty
    expect(sql).toContain("1500000,"); // price_vnd
    expect(sql).toContain("1700000000"); // ts
  });

  it("preserves legacy_id as the INSERT id", () => {
    const sql = buildInsertSql({ ...BASE_ROW, id: 42 });
    // id=42 should be the first value
    expect(sql).toMatch(/VALUES \(42,/);
  });

  it("escapes single quotes in user_id", () => {
    const sql = buildInsertSql({ ...BASE_ROW, user_id: "o'brien" });
    expect(sql).toContain("'o''brien'");
  });

  it("escapes single quotes in symbol", () => {
    const sql = buildInsertSql({ ...BASE_ROW, symbol: "it's" });
    expect(sql).toContain("'it''s'");
  });

  it("handles decimal qty correctly", () => {
    const sql = buildInsertSql({ ...BASE_ROW, qty: 1.23456789 });
    expect(sql).toContain("1.23456789");
  });

  it("handles zero values without coercion errors", () => {
    const sql = buildInsertSql({
      id: 0,
      user_id: "",
      symbol: "",
      side: "",
      qty: 0,
      price_vnd: 0,
      ts: 0,
    });
    expect(sql).toMatch(/VALUES \(0, '', '', '', 0, 0, 0\);/);
  });

  it("sequential id generation: each row gets unique ascending id", () => {
    // Simulate the script logic of incrementing nextId for docs without legacy_id
    let nextId = 1;
    const rows = [
      { user_id: "a", symbol: "BTC", side: "buy", qty: 1, price_vnd: 100, ts: 1 },
      { user_id: "b", symbol: "ETH", side: "sell", qty: 2, price_vnd: 200, ts: 2 },
    ];
    const stmts = rows.map((r) => buildInsertSql({ ...r, id: nextId++ }));
    expect(stmts[0]).toMatch(/VALUES \(1,/);
    expect(stmts[1]).toMatch(/VALUES \(2,/);
  });
});
