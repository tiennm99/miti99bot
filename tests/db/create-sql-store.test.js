import { describe, expect, it } from "vitest";
import { createSqlStore } from "../../src/db/create-sql-store.js";
import { makeFakeD1 } from "../fakes/fake-d1.js";

const makeEnv = () => ({ DB: makeFakeD1() });

describe("createSqlStore", () => {
  describe("factory validation", () => {
    it("throws on missing moduleName", () => {
      expect(() => createSqlStore("", makeEnv())).toThrow(/moduleName is required/);
      expect(() => createSqlStore(null, makeEnv())).toThrow(/moduleName is required/);
    });

    it("throws on invalid moduleName characters", () => {
      expect(() => createSqlStore("Bad Name", makeEnv())).toThrow(/invalid moduleName/);
      expect(() => createSqlStore("has space", makeEnv())).toThrow(/invalid moduleName/);
    });

    it("returns null when env.DB is absent", () => {
      expect(createSqlStore("mymod", {})).toBeNull();
      expect(createSqlStore("mymod", { KV: {} })).toBeNull();
    });

    it("returns a SqlStore when env.DB is present", () => {
      const sql = createSqlStore("mymod", makeEnv());
      expect(sql).not.toBeNull();
      expect(typeof sql.run).toBe("function");
      expect(typeof sql.all).toBe("function");
      expect(typeof sql.first).toBe("function");
      expect(typeof sql.prepare).toBe("function");
      expect(typeof sql.batch).toBe("function");
    });
  });

  describe("tablePrefix", () => {
    it("exposes tablePrefix as moduleName + underscore", () => {
      const sql = createSqlStore("trading", makeEnv());
      expect(sql.tablePrefix).toBe("trading_");
    });

    it("works with hyphenated module names", () => {
      const sql = createSqlStore("my-mod", makeEnv());
      expect(sql.tablePrefix).toBe("my-mod_");
    });
  });

  describe("run", () => {
    it("returns changes and last_row_id on INSERT", async () => {
      const sql = createSqlStore("trading", makeEnv());
      const result = await sql.run("INSERT INTO trading_trades VALUES (?)", "x");
      expect(result).toHaveProperty("changes");
      expect(result).toHaveProperty("last_row_id");
      expect(typeof result.changes).toBe("number");
    });

    it("records the query in runLog", async () => {
      const fakeDb = makeFakeD1();
      const sql = createSqlStore("trading", { DB: fakeDb });
      await sql.run("INSERT INTO trading_trades VALUES (?)", "hello");
      expect(fakeDb.runLog).toHaveLength(1);
      expect(fakeDb.runLog[0].query).toBe("INSERT INTO trading_trades VALUES (?)");
      expect(fakeDb.runLog[0].binds).toEqual(["hello"]);
    });
  });

  describe("all", () => {
    it("returns empty array when table has no rows", async () => {
      const sql = createSqlStore("trading", makeEnv());
      const rows = await sql.all("SELECT * FROM trading_trades");
      expect(rows).toEqual([]);
    });

    it("returns seeded rows", async () => {
      const fakeDb = makeFakeD1();
      fakeDb.seed("trading_trades", [
        { id: 1, symbol: "VNM" },
        { id: 2, symbol: "FPT" },
      ]);
      const sql = createSqlStore("trading", { DB: fakeDb });
      const rows = await sql.all("SELECT * FROM trading_trades");
      expect(rows).toHaveLength(2);
      expect(rows[0].symbol).toBe("VNM");
    });
  });

  describe("first", () => {
    it("returns null when table is empty", async () => {
      const sql = createSqlStore("trading", makeEnv());
      const row = await sql.first("SELECT * FROM trading_trades WHERE id = ?", 99);
      expect(row).toBeNull();
    });

    it("returns the first seeded row", async () => {
      const fakeDb = makeFakeD1();
      fakeDb.seed("trading_trades", [{ id: 1, symbol: "VNM" }]);
      const sql = createSqlStore("trading", { DB: fakeDb });
      const row = await sql.first("SELECT * FROM trading_trades LIMIT 1");
      expect(row).toEqual({ id: 1, symbol: "VNM" });
    });
  });

  describe("batch", () => {
    it("executes multiple statements and returns array of result arrays", async () => {
      const fakeDb = makeFakeD1();
      fakeDb.seed("trading_trades", [{ id: 1 }]);
      const sql = createSqlStore("trading", { DB: fakeDb });
      const stmt1 = sql.prepare("SELECT * FROM trading_trades");
      const stmt2 = sql.prepare("SELECT * FROM trading_orders");
      const results = await sql.batch([stmt1, stmt2]);
      expect(Array.isArray(results)).toBe(true);
      expect(results).toHaveLength(2);
      expect(results[0]).toHaveLength(1); // one row seeded
      expect(results[1]).toHaveLength(0); // no rows
    });
  });
});
