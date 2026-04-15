import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  buildRegistry,
  getCurrentRegistry,
  loadModules,
  resetRegistry,
} from "../../src/modules/registry.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";
import { makeCommand, makeFakeImportMap, makeModule } from "../fakes/fake-modules.js";

const makeEnv = (modules) => ({ MODULES: modules, KV: makeFakeKv() });

describe("registry", () => {
  beforeEach(() => resetRegistry());

  describe("loadModules", () => {
    it("loads modules listed in env.MODULES", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("foo", "public")]),
        b: makeModule("b", [makeCommand("bar", "public")]),
      });
      const mods = await loadModules({ MODULES: "a,b" }, map);
      expect(mods.map((m) => m.name)).toEqual(["a", "b"]);
    });

    it("trims, dedupes, skips empty", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("foo", "public")]),
      });
      const mods = await loadModules({ MODULES: " a , a , " }, map);
      expect(mods).toHaveLength(1);
    });

    it("throws on empty MODULES", async () => {
      await expect(loadModules({ MODULES: "" }, {})).rejects.toThrow(/empty/);
      await expect(loadModules({}, {})).rejects.toThrow(/empty/);
    });

    it("throws on unknown module name", async () => {
      await expect(loadModules({ MODULES: "ghost" }, {})).rejects.toThrow(/unknown module: ghost/);
    });

    it("throws when default export name mismatches key", async () => {
      const map = makeFakeImportMap({ a: makeModule("b", []) });
      await expect(loadModules({ MODULES: "a" }, map)).rejects.toThrow(/mismatched name/);
    });

    it("throws when module has no commands array", async () => {
      const map = {
        a: async () => ({ default: { name: "a" } }),
      };
      await expect(loadModules({ MODULES: "a" }, map)).rejects.toThrow(/commands array/);
    });

    it("runs validateCommand on each command", async () => {
      const badMap = makeFakeImportMap({
        a: makeModule("a", [
          { name: "/bad", visibility: "public", description: "x", handler: async () => {} },
        ]),
      });
      await expect(loadModules({ MODULES: "a" }, badMap)).rejects.toThrow();
    });
  });

  describe("buildRegistry", () => {
    it("partitions commands across visibility maps + flat allCommands", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [
          makeCommand("pub", "public"),
          makeCommand("prot", "protected"),
          makeCommand("priv", "private"),
        ]),
      });
      const reg = await buildRegistry(makeEnv("a"), map);
      expect([...reg.publicCommands.keys()]).toEqual(["pub"]);
      expect([...reg.protectedCommands.keys()]).toEqual(["prot"]);
      expect([...reg.privateCommands.keys()]).toEqual(["priv"]);
      expect([...reg.allCommands.keys()].sort()).toEqual(["priv", "prot", "pub"]);
    });

    it("throws on same-visibility conflict", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("foo", "public")]),
        b: makeModule("b", [makeCommand("foo", "public")]),
      });
      await expect(buildRegistry(makeEnv("a,b"), map)).rejects.toThrow(/conflict.*foo.*a.*b/);
    });

    it("throws on CROSS-visibility conflict (unified namespace)", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("foo", "public")]),
        b: makeModule("b", [makeCommand("foo", "private")]),
      });
      await expect(buildRegistry(makeEnv("a,b"), map)).rejects.toThrow(/conflict.*foo/);
    });

    it("calls init({db, env}) exactly once per module, in order", async () => {
      const calls = [];
      const makeInit =
        (name) =>
        async ({ db, env }) => {
          calls.push({ name, hasDb: !!db, hasEnv: !!env });
          await db.put("sentinel", "x");
        };
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("one", "public")], makeInit("a")),
        b: makeModule("b", [makeCommand("two", "public")], makeInit("b")),
      });
      const env = makeEnv("a,b");
      await buildRegistry(env, map);
      expect(calls).toEqual([
        { name: "a", hasDb: true, hasEnv: true },
        { name: "b", hasDb: true, hasEnv: true },
      ]);
      // Assert prefixing: keys landed as "a:sentinel" and "b:sentinel".
      expect(env.KV.store.get("a:sentinel")).toBe("x");
      expect(env.KV.store.get("b:sentinel")).toBe("x");
    });

    it("wraps init errors with module name", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("foo", "public")], async () => {
          throw new Error("boom");
        }),
      });
      await expect(buildRegistry(makeEnv("a"), map)).rejects.toThrow(
        /module "a" init failed.*boom/,
      );
    });
  });

  describe("cron collection", () => {
    const makeCron = (name, schedule = "0 1 * * *") => ({
      name,
      schedule,
      handler: async () => {},
    });

    it("registry.crons is empty when no modules declare crons", async () => {
      const map = makeFakeImportMap({ a: makeModule("a", [makeCommand("foo", "public")]) });
      const reg = await buildRegistry(makeEnv("a"), map);
      expect(reg.crons).toEqual([]);
    });

    it("collects crons from modules that declare them", async () => {
      const modWithCron = {
        ...makeModule("a", [makeCommand("foo", "public")]),
        crons: [makeCron("nightly")],
      };
      const map = makeFakeImportMap({ a: modWithCron });
      const reg = await buildRegistry(makeEnv("a"), map);
      expect(reg.crons).toHaveLength(1);
      expect(reg.crons[0].name).toBe("nightly");
      expect(reg.crons[0].schedule).toBe("0 1 * * *");
      expect(reg.crons[0].module.name).toBe("a");
    });

    it("fan-out: two modules with same schedule both appear in registry.crons", async () => {
      const modA = {
        ...makeModule("a", [makeCommand("ca", "public")]),
        crons: [makeCron("tick", "*/5 * * * *")],
      };
      const modB = {
        ...makeModule("b", [makeCommand("cb", "public")]),
        crons: [makeCron("tick", "*/5 * * * *")],
      };
      const map = makeFakeImportMap({ a: modA, b: modB });
      const reg = await buildRegistry(makeEnv("a,b"), map);
      expect(reg.crons).toHaveLength(2);
      expect(reg.crons.map((c) => c.module.name).sort()).toEqual(["a", "b"]);
    });

    it("throws on duplicate cron name within the same module", async () => {
      const mod = {
        ...makeModule("a", [makeCommand("foo", "public")]),
        crons: [makeCron("dup"), makeCron("dup")],
      };
      const map = makeFakeImportMap({ a: mod });
      await expect(buildRegistry(makeEnv("a"), map)).rejects.toThrow(/duplicate cron name "dup"/);
    });

    it("throws when crons is not an array", async () => {
      const mod = { ...makeModule("a", [makeCommand("foo", "public")]), crons: "bad" };
      const map = makeFakeImportMap({ a: mod });
      await expect(buildRegistry(makeEnv("a"), map)).rejects.toThrow(/crons must be an array/);
    });

    it("throws when a cron entry fails validation", async () => {
      const mod = {
        ...makeModule("a", [makeCommand("foo", "public")]),
        crons: [{ name: "bad!", schedule: "0 1 * * *", handler: async () => {} }],
      };
      const map = makeFakeImportMap({ a: mod });
      await expect(buildRegistry(makeEnv("a"), map)).rejects.toThrow(/name must match/);
    });
  });

  describe("getCurrentRegistry / resetRegistry", () => {
    it("getCurrentRegistry throws before build", () => {
      expect(() => getCurrentRegistry()).toThrow(/not built/);
    });

    it("getCurrentRegistry returns the same instance after build", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("foo", "public")]),
      });
      const reg = await buildRegistry(makeEnv("a"), map);
      expect(getCurrentRegistry()).toBe(reg);
    });

    it("resetRegistry clears the memoized registry", async () => {
      const map = makeFakeImportMap({
        a: makeModule("a", [makeCommand("foo", "public")]),
      });
      await buildRegistry(makeEnv("a"), map);
      resetRegistry();
      expect(() => getCurrentRegistry()).toThrow(/not built/);
    });
  });
});
