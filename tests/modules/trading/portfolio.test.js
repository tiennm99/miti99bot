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
  /** @type {import("../../../src/db/kv-store-interface.js").KVStore} */
  let db;

  beforeEach(() => {
    db = createStore("trading", { KV: makeFakeKv() });
  });

  describe("emptyPortfolio", () => {
    it("returns correct shape", () => {
      const p = emptyPortfolio();
      expect(p.currency).toEqual({ VND: 0, USD: 0 });
      expect(p.stock).toEqual({});
      expect(p.crypto).toEqual({});
      expect(p.others).toEqual({});
      expect(p.totalvnd).toBe(0);
    });
  });

  describe("getPortfolio / savePortfolio", () => {
    it("returns empty portfolio for new user", async () => {
      const p = await getPortfolio(db, 123);
      expect(p.currency.VND).toBe(0);
      expect(p.totalvnd).toBe(0);
    });

    it("round-trips saved data", async () => {
      const p = emptyPortfolio();
      p.currency.VND = 5000000;
      p.totalvnd = 5000000;
      await savePortfolio(db, 123, p);
      const loaded = await getPortfolio(db, 123);
      expect(loaded.currency.VND).toBe(5000000);
      expect(loaded.totalvnd).toBe(5000000);
    });

    it("fills missing category keys on load (migration-safe)", async () => {
      // simulate old data missing 'others' key
      await db.putJSON("user:123", { currency: { VND: 100 }, stock: {}, crypto: {} });
      const p = await getPortfolio(db, 123);
      expect(p.others).toEqual({});
      expect(p.totalvnd).toBe(0);
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
      expect(p.currency.VND).toBe(1000); // unchanged
    });
  });

  describe("addAsset / deductAsset", () => {
    it("adds crypto asset", () => {
      const p = emptyPortfolio();
      addAsset(p, "BTC", 0.5);
      expect(p.crypto.BTC).toBe(0.5);
    });

    it("adds stock asset", () => {
      const p = emptyPortfolio();
      addAsset(p, "TCB", 10);
      expect(p.stock.TCB).toBe(10);
    });

    it("adds others asset", () => {
      const p = emptyPortfolio();
      addAsset(p, "GOLD", 1);
      expect(p.others.GOLD).toBe(1);
    });

    it("accumulates on repeated add", () => {
      const p = emptyPortfolio();
      addAsset(p, "BTC", 0.1);
      addAsset(p, "BTC", 0.2);
      expect(p.crypto.BTC).toBeCloseTo(0.3);
    });

    it("deducts asset when sufficient", () => {
      const p = emptyPortfolio();
      p.crypto.BTC = 1.0;
      const result = deductAsset(p, "BTC", 0.3);
      expect(result.ok).toBe(true);
      expect(p.crypto.BTC).toBeCloseTo(0.7);
    });

    it("removes key when deducted to zero", () => {
      const p = emptyPortfolio();
      p.crypto.BTC = 0.5;
      deductAsset(p, "BTC", 0.5);
      expect(p.crypto.BTC).toBeUndefined();
    });

    it("rejects deduction when insufficient", () => {
      const p = emptyPortfolio();
      p.crypto.BTC = 0.1;
      const result = deductAsset(p, "BTC", 0.5);
      expect(result.ok).toBe(false);
      expect(result.held).toBe(0.1);
    });

    it("rejects deduction for unknown symbol", () => {
      const p = emptyPortfolio();
      const result = deductAsset(p, "NOPE", 1);
      expect(result.ok).toBe(false);
    });
  });
});
