import { describe, expect, it } from "vitest";
import {
  CURRENCIES,
  SYMBOLS,
  getSymbol,
  listSymbols,
} from "../../../src/modules/trading/symbols.js";

describe("trading/symbols", () => {
  it("SYMBOLS has 9 entries across 3 categories", () => {
    expect(Object.keys(SYMBOLS)).toHaveLength(9);
    const cats = new Set(Object.values(SYMBOLS).map((s) => s.category));
    expect(cats).toEqual(new Set(["crypto", "stock", "others"]));
  });

  it("getSymbol is case-insensitive", () => {
    const btc = getSymbol("btc");
    expect(btc).toBeDefined();
    expect(btc.symbol).toBe("BTC");
    expect(btc.category).toBe("crypto");

    expect(getSymbol("Tcb").symbol).toBe("TCB");
  });

  it("getSymbol returns undefined for unknown symbols", () => {
    expect(getSymbol("NOPE")).toBeUndefined();
    expect(getSymbol("")).toBeUndefined();
    expect(getSymbol(null)).toBeUndefined();
  });

  it("CURRENCIES contains VND and USD", () => {
    expect(CURRENCIES.has("VND")).toBe(true);
    expect(CURRENCIES.has("USD")).toBe(true);
    expect(CURRENCIES.has("EUR")).toBe(false);
  });

  it("listSymbols returns grouped output", () => {
    const out = listSymbols();
    expect(out).toContain("Crypto:");
    expect(out).toContain("BTC — Bitcoin");
    expect(out).toContain("Stocks:");
    expect(out).toContain("TCB — Techcombank");
    expect(out).toContain("Others:");
    expect(out).toContain("GOLD — Gold");
  });
});
