import { beforeEach, describe, expect, it } from "vitest";
import { installDispatcher } from "../../src/modules/dispatcher.js";
import { resetRegistry } from "../../src/modules/registry.js";
import { makeFakeBot } from "../fakes/fake-bot.js";
import { makeFakeKv } from "../fakes/fake-kv-namespace.js";
import { makeCommand, makeFakeImportMap, makeModule } from "../fakes/fake-modules.js";

// The dispatcher imports the registry's default static map at module-load
// time, so to keep the test hermetic we need to stub env.MODULES with names
// that resolve in the REAL map — OR we call buildRegistry directly with an
// injected map and then simulate what installDispatcher does. Since phase-04
// made loadModules accept an importMap injection but installDispatcher
// internally calls buildRegistry WITHOUT an injection, we test the same
// effect by wiring the bot manually against a registry built with a fake map.
//
// Keeping it simple: call installDispatcher against the real module map
// (util + wordle + loldle + misc) so we exercise the actual production path.

describe("installDispatcher", () => {
  beforeEach(() => resetRegistry());

  it("registers EVERY command via bot.command() regardless of visibility", async () => {
    const bot = makeFakeBot();
    const env = { MODULES: "util,wordle,loldle,misc", KV: makeFakeKv() };
    const reg = await installDispatcher(bot, env);

    // Expect 11 total commands (7 public + 2 protected + 2 private).
    expect(bot.commandCalls).toHaveLength(11);
    expect(reg.allCommands.size).toBe(11);

    const registeredNames = bot.commandCalls.map((c) => c.name).sort();
    const expected = [...reg.allCommands.keys()].sort();
    expect(registeredNames).toEqual(expected);

    // Assert private commands ARE registered (the whole point of unified routing).
    expect(registeredNames).toContain("konami");
    expect(registeredNames).toContain("fortytwo");
  });

  it("does NOT install any bot.on() middleware — dispatcher is minimal", async () => {
    const bot = makeFakeBot();
    const env = { MODULES: "util", KV: makeFakeKv() };
    await installDispatcher(bot, env);
    expect(bot.onCalls).toHaveLength(0);
  });

  it("each registered handler matches the source module command handler", async () => {
    const bot = makeFakeBot();
    const env = { MODULES: "util", KV: makeFakeKv() };
    const reg = await installDispatcher(bot, env);
    for (const { name, handler } of bot.commandCalls) {
      expect(handler).toBe(reg.allCommands.get(name).cmd.handler);
    }
  });
});
