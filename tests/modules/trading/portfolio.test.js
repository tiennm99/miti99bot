import { beforeEach, describe, expect, it } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import {
  addAsset,
  addCurrency,
  deductAsset,
  deductCurrency,
  emptyPortfolio,
  getPortfolio,
  savePortfolio,
} from "../../../src/modules/trading/portfolio.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

describe("trading/portfolio", () => {
  let db;

  beforeEach(() => {
    db = createStore("trading", { KV: makeFakeKv() });
  });

  describe("emptyPortfolio", () => {
    it("returns correct shape", () => {
      const p = emptyPortfolio();
      expect(p.currency).toEqual({ VND: 0 });
      expect(p.assets).toEqual({});
      expect(p.totalvnd).toBe(0);
    });
  });

  describe("getPortfolio / savePortfolio", () => {
    it("returns empty portfolio for new user", async () => {
      const p = await getPortfolio(db, 123);
      expect(p.currency.VND).toBe(0);
      expect(p.totalvnd).toBe(0);
      expect(p.assets).toEqual({});
    });

    it("round-trips saved data", async () => {
      const p = emptyPortfolio();
      p.currency.VND = 5000000;
      p.assets.TCB = 10;
      p.totalvnd = 5000000;
      await savePortfolio(db, 123, p);
      const loaded = await getPortfolio(db, 123);
      expect(loaded.currency.VND).toBe(5000000);
      expect(loaded.assets.TCB).toBe(10);
      expect(loaded.totalvnd).toBe(5000000);
    });

    it("migrates old 4-category format to flat assets", async () => {
      // simulate old format with stock/crypto/others
      await db.putJSON("user:123", {
        currency: { VND: 100 },
        stock: { TCB: 10 },
        crypto: { BTC: 0.5 },
        others: { GOLD: 1 },
        totalvnd: 100,
      });
      const p = await getPortfolio(db, 123);
      expect(p.assets.TCB).toBe(10);
      expect(p.assets.BTC).toBe(0.5);
      expect(p.assets.GOLD).toBe(1);
      // old category keys should not exist
      expect(p.stock).toBeUndefined();
      expect(p.crypto).toBeUndefined();
    });
  });

  describe("addCurrency / deductCurrency", () => {
    it("adds currency", () => {
      const p = emptyPortfolio();
      addCurrency(p, "VND", 1000000);
      expect(p.currency.VND).toBe(1000000);
    });

    it("deducts currency when sufficient", () => {
      const p = emptyPortfolio();
      p.currency.VND = 5000000;
      const result = deductCurrency(p, "VND", 3000000);
      expect(result.ok).toBe(true);
      expect(p.currency.VND).toBe(2000000);
    });

    it("rejects deduction when insufficient", () => {
      const p = emptyPortfolio();
      p.currency.VND = 1000;
      const result = deductCurrency(p, "VND", 5000);
      expect(result.ok).toBe(false);
      expect(result.balance).toBe(1000);
    });
  });

  describe("addAsset / deductAsset", () => {
    it("adds asset to flat map", () => {
      const p = emptyPortfolio();
      addAsset(p, "TCB", 10);
      expect(p.assets.TCB).toBe(10);
    });

    it("accumulates on repeated add", () => {
      const p = emptyPortfolio();
      addAsset(p, "TCB", 10);
      addAsset(p, "TCB", 5);
      expect(p.assets.TCB).toBe(15);
    });

    it("deducts asset when sufficient", () => {
      const p = emptyPortfolio();
      p.assets.TCB = 10;
      const result = deductAsset(p, "TCB", 3);
      expect(result.ok).toBe(true);
      expect(p.assets.TCB).toBe(7);
    });

    it("removes key when deducted to zero", () => {
      const p = emptyPortfolio();
      p.assets.TCB = 5;
      deductAsset(p, "TCB", 5);
      expect(p.assets.TCB).toBeUndefined();
    });

    it("rejects deduction when insufficient", () => {
      const p = emptyPortfolio();
      p.assets.TCB = 3;
      const result = deductAsset(p, "TCB", 10);
      expect(result.ok).toBe(false);
      expect(result.held).toBe(3);
    });

    it("rejects deduction for unowned symbol", () => {
      const p = emptyPortfolio();
      const result = deductAsset(p, "NOPE", 1);
      expect(result.ok).toBe(false);
      expect(result.held).toBe(0);
    });
  });
});
