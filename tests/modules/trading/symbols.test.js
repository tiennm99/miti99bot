import { beforeEach, describe, expect, it, vi } from "vitest";
import { createStore } from "../../../src/db/create-store.js";
import { comingSoonMessage, resolveSymbol } from "../../../src/modules/trading/symbols.js";
import { makeFakeKv } from "../../fakes/fake-kv-namespace.js";

/** Stub global.fetch to return TCBS-like response */
function stubFetch(hasData = true) {
  global.fetch = vi.fn(() =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve(hasData ? { data: [{ close: 25 }] } : { data: [] }),
    }),
  );
}

describe("trading/symbols", () => {
  let db;

  beforeEach(() => {
    db = createStore("trading", { KV: makeFakeKv() });
    vi.restoreAllMocks();
  });

  it("resolves a valid VN stock ticker via TCBS", async () => {
    stubFetch();
    const result = await resolveSymbol(db, "TCB");
    expect(result).toEqual({ symbol: "TCB", category: "stock", label: "TCB" });
    expect(global.fetch).toHaveBeenCalledOnce();
  });

  it("caches resolved symbol in KV", async () => {
    stubFetch();
    await resolveSymbol(db, "TCB");
    // second call should use cache, not fetch
    await resolveSymbol(db, "TCB");
    expect(global.fetch).toHaveBeenCalledOnce();
  });

  it("is case-insensitive", async () => {
    stubFetch();
    const result = await resolveSymbol(db, "tcb");
    expect(result.symbol).toBe("TCB");
  });

  it("returns null for invalid ticker", async () => {
    stubFetch(false);
    const result = await resolveSymbol(db, "NOPE");
    expect(result).toBeNull();
  });

  it("returns null for empty input", async () => {
    const result = await resolveSymbol(db, "");
    expect(result).toBeNull();
  });

  it("comingSoonMessage returns string", () => {
    expect(comingSoonMessage()).toContain("coming soon");
  });
});
