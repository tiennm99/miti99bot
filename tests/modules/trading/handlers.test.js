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

/** Build a fake grammY context with .match, .from, and .reply spy */
function makeCtx(match = "", userId = 42) {
  const replies = [];
  return {
    match,
    from: { id: userId },
    reply: vi.fn((text) => replies.push(text)),
    replies,
  };
}

/** Stub global.fetch to return canned price data */
function stubFetch() {
  global.fetch = vi.fn((url) => {
    if (url.includes("coingecko")) {
      return Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            bitcoin: { vnd: 1500000000 },
            ethereum: { vnd: 50000000 },
            solana: { vnd: 3000000 },
            "pax-gold": { vnd: 72000000 },
          }),
      });
    }
    if (url.includes("tcbs")) {
      const ticker = url.match(/ticker=(\w+)/)?.[1];
      const prices = { TCB: 25, VPB: 18, FPT: 120, VNM: 70, HPG: 28 };
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ data: [{ close: prices[ticker] || 25 }] }),
      });
    }
    if (url.includes("er-api")) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ rates: { VND: 25400 } }),
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
      const ctx = makeCtx("5000000 VND");
      await handleTopup(ctx, db);
      expect(ctx.reply).toHaveBeenCalledOnce();
      expect(ctx.replies[0]).toContain("5.000.000 VND");
    });

    it("tops up USD and tracks VND equivalent in totalvnd", async () => {
      const ctx = makeCtx("100 USD");
      await handleTopup(ctx, db);
      expect(ctx.replies[0]).toContain("$100.00");
    });

    it("defaults to VND when currency omitted", async () => {
      const ctx = makeCtx("1000000");
      await handleTopup(ctx, db);
      expect(ctx.replies[0]).toContain("VND");
    });

    it("rejects missing args", async () => {
      const ctx = makeCtx("");
      await handleTopup(ctx, db);
      expect(ctx.replies[0]).toContain("Usage:");
    });

    it("rejects negative amount", async () => {
      const ctx = makeCtx("-100 VND");
      await handleTopup(ctx, db);
      expect(ctx.replies[0]).toContain("positive");
    });

    it("rejects unsupported currency", async () => {
      const ctx = makeCtx("100 EUR");
      await handleTopup(ctx, db);
      expect(ctx.replies[0]).toContain("Unsupported");
    });
  });

  describe("handleBuy", () => {
    it("buys crypto with sufficient VND", async () => {
      // seed VND balance
      const { emptyPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = emptyPortfolio();
      p.currency.VND = 20000000000;
      p.totalvnd = 20000000000;
      await savePortfolio(db, 42, p);

      const ctx = makeCtx("0.01 BTC");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("Bought");
      expect(ctx.replies[0]).toContain("BTC");
    });

    it("rejects buy with insufficient VND", async () => {
      const ctx = makeCtx("1 BTC");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("Insufficient VND");
    });

    it("rejects unknown symbol", async () => {
      const ctx = makeCtx("1 NOPE");
      await handleBuy(ctx, db);
      expect(ctx.replies[0]).toContain("Unknown symbol");
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
    it("sells crypto holding", async () => {
      const { emptyPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = emptyPortfolio();
      p.crypto.BTC = 0.5;
      await savePortfolio(db, 42, p);

      const ctx = makeCtx("0.1 BTC");
      await handleSell(ctx, db);
      expect(ctx.replies[0]).toContain("Sold");
      expect(ctx.replies[0]).toContain("BTC");
    });

    it("rejects sell with insufficient holdings", async () => {
      const ctx = makeCtx("1 BTC");
      await handleSell(ctx, db);
      expect(ctx.replies[0]).toContain("Insufficient BTC");
    });
  });

  describe("handleConvert", () => {
    it("converts USD to VND", async () => {
      const { emptyPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = emptyPortfolio();
      p.currency.USD = 100;
      await savePortfolio(db, 42, p);

      const ctx = makeCtx("50 USD VND");
      await handleConvert(ctx, db);
      expect(ctx.replies[0]).toContain("Converted");
      expect(ctx.replies[0]).toContain("VND");
    });

    it("rejects same currency conversion", async () => {
      const ctx = makeCtx("100 VND VND");
      await handleConvert(ctx, db);
      expect(ctx.replies[0]).toContain("same currency");
    });

    it("rejects insufficient balance", async () => {
      const ctx = makeCtx("100 USD VND");
      await handleConvert(ctx, db);
      expect(ctx.replies[0]).toContain("Insufficient USD");
    });
  });

  describe("handleStats", () => {
    it("shows empty portfolio for new user", async () => {
      const ctx = makeCtx("");
      await handleStats(ctx, db);
      expect(ctx.replies[0]).toContain("Portfolio Summary");
      expect(ctx.replies[0]).toContain("Total value:");
    });

    it("shows portfolio with assets", async () => {
      const { emptyPortfolio } = await import("../../../src/modules/trading/portfolio.js");
      const p = emptyPortfolio();
      p.currency.VND = 5000000;
      p.crypto.BTC = 0.01;
      p.totalvnd = 20000000;
      await savePortfolio(db, 42, p);

      const ctx = makeCtx("");
      await handleStats(ctx, db);
      expect(ctx.replies[0]).toContain("BTC");
      expect(ctx.replies[0]).toContain("P&L:");
    });
  });
});
