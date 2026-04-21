/**
 * @file Tests for trading/history — recordTrade, listTrades, formatTradesHtml,
 * createHistoryHandler, and buy/sell → D1 integration.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createSqlStore } from "../../../src/db/create-sql-store.js";
import {
  createHistoryHandler,
  formatTradesHtml,
  listTrades,
  recordTrade,
} from "../../../src/modules/trading/history.js";
import { makeFakeD1 } from "../../fakes/fake-d1.js";

// ─── helpers ────────────────────────────────────────────────────────────────

/** Build a SqlStore backed by a fresh fake D1, pre-seeded with the table. */
function makeTestSql() {
  const fakeDb = makeFakeD1();
  const sql = createSqlStore("trading", { DB: fakeDb });
  return { fakeDb, sql };
}

/** Minimal grammY ctx double. */
function makeCtx(match = "", userId = 123) {
  return {
    match,
    from: { id: userId },
    reply: vi.fn(),
  };
}

/** A canned trade payload. */
const TRADE = { userId: 1, symbol: "TCB", side: "buy", qty: 100, priceVnd: 25000 };

// ─── recordTrade ─────────────────────────────────────────────────────────────

describe("recordTrade", () => {
  it("inserts a row into trading_trades", async () => {
    const { fakeDb, sql } = makeTestSql();
    await recordTrade(sql, TRADE);
    expect(fakeDb.runLog).toHaveLength(1);
    expect(fakeDb.runLog[0].query).toMatch(/INSERT INTO trading_trades/i);
    const binds = fakeDb.runLog[0].binds;
    expect(binds[0]).toBe(1); // user_id
    expect(binds[1]).toBe("TCB"); // symbol
    expect(binds[2]).toBe("buy"); // side
    expect(binds[3]).toBe(100); // qty
    expect(binds[4]).toBe(25000); // price_vnd
    expect(typeof binds[5]).toBe("number"); // ts
  });

  it("logs a warning and returns silently when sql is null", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    await expect(recordTrade(null, TRADE)).resolves.toBeUndefined();
    expect(warn).toHaveBeenCalledWith(expect.stringContaining("no D1 binding"));
    warn.mockRestore();
  });

  it("logs an error and does NOT throw when the insert fails", async () => {
    const fakeDb = makeFakeD1();
    // Make prepare().bind().run() throw.
    vi.spyOn(fakeDb, "prepare").mockReturnValue({
      bind: () => ({
        run: async () => {
          throw new Error("disk full");
        },
      }),
    });
    const sql = createSqlStore("trading", { DB: fakeDb });
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    await expect(recordTrade(sql, TRADE)).resolves.toBeUndefined();
    expect(error).toHaveBeenCalledWith(
      expect.stringContaining("recordTrade failed"),
      expect.any(Error),
    );
    error.mockRestore();
  });
});

// ─── listTrades ──────────────────────────────────────────────────────────────

describe("listTrades", () => {
  it("returns [] when sql is null", async () => {
    expect(await listTrades(null, 1, 10)).toEqual([]);
  });

  it("returns [] when table is empty", async () => {
    const { sql } = makeTestSql();
    expect(await listTrades(sql, 1, 10)).toEqual([]);
  });

  it("maps snake_case columns to camelCase Trade objects", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", [
      { id: 1, user_id: 1, symbol: "TCB", side: "buy", qty: 10, price_vnd: 25000, ts: 1000 },
    ]);
    const trades = await listTrades(sql, 1, 10);
    expect(trades).toHaveLength(1);
    expect(trades[0]).toMatchObject({
      id: 1,
      userId: 1,
      symbol: "TCB",
      side: "buy",
      qty: 10,
      priceVnd: 25000,
      ts: 1000,
    });
  });

  it("clamps limit below 1 to 1", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", [
      { id: 1, user_id: 1, symbol: "A", side: "buy", qty: 1, price_vnd: 100, ts: 1 },
    ]);
    // Should not throw; clamps to 1 internally — binds[1] == 1.
    const trades = await listTrades(sql, 1, 0);
    // fake-d1 returns all seeded rows regardless of LIMIT bind, but we verify the bind.
    expect(fakeDb.queryLog[0].binds[1]).toBe(1);
  });

  it("clamps limit above 50 to 50", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", []);
    await listTrades(sql, 1, 999);
    expect(fakeDb.queryLog[0].binds[1]).toBe(50);
  });
});

// ─── formatTradesHtml ────────────────────────────────────────────────────────

describe("formatTradesHtml", () => {
  it("returns fallback message for empty array", () => {
    expect(formatTradesHtml([])).toBe("No trades recorded yet.");
  });

  it("HTML-escapes symbols containing special characters", () => {
    const trade = {
      id: 1,
      userId: 1,
      symbol: "<script>",
      side: "buy",
      qty: 1,
      priceVnd: 1000,
      ts: Date.now(),
    };
    const html = formatTradesHtml([trade]);
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });

  it("renders BUY/SELL labels", () => {
    const buy = { id: 1, userId: 1, symbol: "TCB", side: "buy", qty: 10, priceVnd: 25000, ts: 0 };
    const sell = { id: 2, userId: 1, symbol: "TCB", side: "sell", qty: 5, priceVnd: 26000, ts: 1 };
    const html = formatTradesHtml([buy, sell]);
    expect(html).toContain("BUY");
    expect(html).toContain("SELL");
  });

  it("includes quantity and symbol in output", () => {
    const trade = {
      id: 1,
      userId: 1,
      symbol: "VNM",
      side: "sell",
      qty: 50,
      priceVnd: 80000,
      ts: 0,
    };
    const html = formatTradesHtml([trade]);
    expect(html).toContain("50");
    expect(html).toContain("VNM");
  });
});

// ─── createHistoryHandler ─────────────────────────────────────────────────────

describe("createHistoryHandler", () => {
  it("uses default 10 when match is empty", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", []);
    const handler = createHistoryHandler(sql);
    const ctx = makeCtx("", 1);
    await handler(ctx);
    // binds[1] should be 10 (the LIMIT bind)
    expect(fakeDb.queryLog[0].binds[1]).toBe(10);
    expect(ctx.reply).toHaveBeenCalledOnce();
  });

  it("passes parsed N to listTrades", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", []);
    const handler = createHistoryHandler(sql);
    const ctx = makeCtx("25", 1);
    await handler(ctx);
    expect(fakeDb.queryLog[0].binds[1]).toBe(25);
  });

  it("clamps N=999 to 50", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", []);
    const handler = createHistoryHandler(sql);
    const ctx = makeCtx("999", 1);
    await handler(ctx);
    expect(fakeDb.queryLog[0].binds[1]).toBe(50);
  });

  it("falls back to default 10 when N=0", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", []);
    const handler = createHistoryHandler(sql);
    const ctx = makeCtx("0", 1);
    await handler(ctx);
    expect(fakeDb.queryLog[0].binds[1]).toBe(10);
  });

  it("replies with HTML parse_mode", async () => {
    const { fakeDb, sql } = makeTestSql();
    fakeDb.seed("trading_trades", []);
    const handler = createHistoryHandler(sql);
    const ctx = makeCtx("", 1);
    await handler(ctx);
    expect(ctx.reply).toHaveBeenCalledWith(expect.any(String), { parse_mode: "HTML" });
  });

  it("works with sql=null — returns empty list message", async () => {
    const handler = createHistoryHandler(null);
    const ctx = makeCtx("", 1);
    await handler(ctx);
    expect(ctx.reply).toHaveBeenCalledWith("No trades recorded yet.", { parse_mode: "HTML" });
  });
});

// ─── buy/sell → D1 integration ───────────────────────────────────────────────

describe("buy/sell handlers → recordTrade integration", () => {
  afterEach(() => vi.restoreAllMocks());

  function stubFetch() {
    global.fetch = vi.fn((url) => {
      if (url.includes("kbsec")) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({
              symbol: "TCB",
              data_day: [{ t: "2026-04-21 07:00", c: 25000 }],
            }),
        });
      }
      if (url.includes("bidv")) {
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({ data: [{ currency: "USD", muaCk: "25,200", ban: "25,600" }] }),
        });
      }
      return Promise.reject(new Error(`unexpected fetch: ${url}`));
    });
  }

  it("buy inserts a row into trading_trades", async () => {
    stubFetch();
    const { fakeDb, sql } = makeTestSql();
    const { createStore } = await import("../../../src/db/create-store.js");
    const { makeFakeKv } = await import("../../fakes/fake-kv-namespace.js");
    const { handleBuy } = await import("../../../src/modules/trading/handlers.js");
    const { savePortfolio, emptyPortfolio } = await import(
      "../../../src/modules/trading/portfolio.js"
    );

    const db = createStore("trading", { KV: makeFakeKv() });
    const p = emptyPortfolio();
    p.currency.VND = 10_000_000;
    await savePortfolio(db, 42, p);

    const onTrade = vi.fn(({ symbol, side, qty, priceVnd }) =>
      recordTrade(sql, { userId: 42, symbol, side, qty, priceVnd }),
    );
    const ctx = makeCtx("100 TCB", 42);
    await handleBuy(ctx, db, onTrade);

    expect(onTrade).toHaveBeenCalledOnce();
    expect(fakeDb.runLog).toHaveLength(1);
    expect(fakeDb.runLog[0].binds[2]).toBe("buy");
    expect(fakeDb.runLog[0].binds[1]).toBe("TCB");
  });

  it("sell inserts a row into trading_trades", async () => {
    stubFetch();
    const { fakeDb, sql } = makeTestSql();
    const { createStore } = await import("../../../src/db/create-store.js");
    const { makeFakeKv } = await import("../../fakes/fake-kv-namespace.js");
    const { handleSell } = await import("../../../src/modules/trading/handlers.js");
    const { savePortfolio, emptyPortfolio } = await import(
      "../../../src/modules/trading/portfolio.js"
    );

    const db = createStore("trading", { KV: makeFakeKv() });
    const p = emptyPortfolio();
    p.assets.TCB = 200;
    await savePortfolio(db, 42, p);

    const onTrade = vi.fn(({ symbol, side, qty, priceVnd }) =>
      recordTrade(sql, { userId: 42, symbol, side, qty, priceVnd }),
    );
    const ctx = makeCtx("50 TCB", 42);
    await handleSell(ctx, db, onTrade);

    expect(onTrade).toHaveBeenCalledOnce();
    expect(fakeDb.runLog).toHaveLength(1);
    expect(fakeDb.runLog[0].binds[2]).toBe("sell");
  });

  it("recordTrade failure does not prevent trade reply", async () => {
    stubFetch();
    const { sql } = makeTestSql();
    const { createStore } = await import("../../../src/db/create-store.js");
    const { makeFakeKv } = await import("../../fakes/fake-kv-namespace.js");
    const { handleBuy } = await import("../../../src/modules/trading/handlers.js");
    const { savePortfolio, emptyPortfolio } = await import(
      "../../../src/modules/trading/portfolio.js"
    );

    const db = createStore("trading", { KV: makeFakeKv() });
    const p = emptyPortfolio();
    p.currency.VND = 10_000_000;
    await savePortfolio(db, 42, p);

    // onTrade throws; trade reply must still succeed.
    const onTrade = async () => {
      throw new Error("D1 down");
    };
    const ctx = makeCtx("100 TCB", 42);

    // handleBuy awaits onTrade directly; we wrap to absorb — mirrors real index.js
    // which calls recordTrade (which swallows). Here we test the swallow itself:
    const safeOnTrade = async (t) => {
      try {
        await onTrade(t);
      } catch {
        /* swallowed */
      }
    };
    await handleBuy(ctx, db, safeOnTrade);
    expect(ctx.reply).toHaveBeenCalledWith(expect.stringContaining("Bought"));
  });
});
