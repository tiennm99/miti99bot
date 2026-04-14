import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  handleBuy,
  handleConvert,
  handleSell,
  handleTopup,
} from "../../../src/modules/trading/handlers.js";
import { savePortfolio } from "../../../src/modules/trading/portfolio.js";
import { handleStats } from "../../../src/modules/trading/stats-handler.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

function makeCtx(match = "", userId = 42) {
  const replies = [];
  return {
    match,
    from: { id: userId },
    reply: vi.fn((text) => replies.push(text)),
    replies,
  };
}

/** Stub fetch — TCBS returns stock data, BIDV returns forex */
function stubFetch() {
  global.fetch = vi.fn((url) => {
    if (url.includes("tcbs")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ data: [{ close: 25 }] }),
      });
    }
    if (url.includes("bidv")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [{ currency: "USD", muaCk: "25,200", ban: "25,600" }],
          }),
      });
    }
    return Promise.reject(new Error(`unexpected fetch: ${url}`));
  });
}

describe("trading/handlers", () => {
  let db;

  beforeEach(() => {
    db = createStore("trading", { KV: makeFakeKv() });
    stubFetch();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("handleTopup", () => {
    it("tops up VND", async () => {
      const ctx = makeCtx("5000000");
      await handleTopup(ctx, db);
      expect(ctx.reply).toHaveBeenCalledOnce();
      expect(ctx.replies[0]).toContain("5.000.000 VND");
    });

    it("tracks totalvnd", async () => {
      const ctx = makeCtx("1000000");
      await handleTopup(ctx, db);
      const { getPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = await getPortfolio(db, 42);
      expect(p.totalvnd).toBe(1000000);
      expect(p.currency.VND).toBe(1000000);
    });

    it("rejects missing args", async () => {
      const ctx = makeCtx("");
      await handleTopup(ctx, db);
      expect(ctx.replies[0]).toContain("Usage:");
    });

    it("rejects negative amount", async () => {
      const ctx = makeCtx("-100");
      await handleTopup(ctx, db);
      expect(ctx.replies[0]).toContain("positive");
    });
  });

  describe("handleBuy", () => {
    it("buys stock with sufficient VND", async () => {
      const { emptyPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = emptyPortfolio();
      p.currency.VND = 5000000;
      p.totalvnd = 5000000;
      await savePortfolio(db, 42, p);

      const ctx = makeCtx("10 TCB");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("Bought");
      expect(ctx.replies[0]).toContain("TCB");
    });

    it("rejects buy with insufficient VND", async () => {
      const ctx = makeCtx("10 TCB");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("Insufficient VND");
    });

    it("rejects unknown ticker", async () => {
      // stub TCBS to return empty data for unknown ticker
      global.fetch = vi.fn(() =>
        Promise.resolve({ ok: true, json: () => Promise.resolve({ data: [] }) }),
      );
      const ctx = makeCtx("10 NOPE");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("Unknown stock ticker");
      expect(ctx.replies[0]).toContain("coming soon");
    });

    it("rejects fractional stock quantity", async () => {
      const ctx = makeCtx("1.5 TCB");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("whole numbers");
    });

    it("rejects missing args", async () => {
      const ctx = makeCtx("");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("Usage:");
    });
  });

  describe("handleSell", () => {
    it("sells stock holding", async () => {
      const { emptyPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = emptyPortfolio();
      p.assets.TCB = 50;
      await savePortfolio(db, 42, p);

      const ctx = makeCtx("10 TCB");
      await handleSell(ctx, db);
      expect(ctx.replies[0]).toContain("Sold");
      expect(ctx.replies[0]).toContain("TCB");
    });

    it("rejects sell with insufficient holdings", async () => {
      const ctx = makeCtx("10 TCB");
      await handleSell(ctx, db);
      expect(ctx.replies[0]).toContain("Insufficient TCB");
    });
  });

  describe("handleConvert", () => {
    it("shows coming soon message", async () => {
      const ctx = makeCtx("100 USD VND");
      await handleConvert(ctx);
      expect(ctx.replies[0]).toContain("coming soon");
    });
  });

  describe("handleStats", () => {
    it("shows empty portfolio for new user", async () => {
      const ctx = makeCtx("");
      await handleStats(ctx, db);
      expect(ctx.replies[0]).toContain("Portfolio Summary");
      expect(ctx.replies[0]).toContain("Total value:");
    });

    it("shows portfolio with stock assets", async () => {
      const { emptyPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = emptyPortfolio();
      p.currency.VND = 5000000;
      p.assets.TCB = 10;
      p.totalvnd = 10000000;
      await savePortfolio(db, 42, p);

      const ctx = makeCtx("");
      await handleStats(ctx, db);
      expect(ctx.replies[0]).toContain("TCB");
      expect(ctx.replies[0]).toContain("P&L:");
    });
  });
});
