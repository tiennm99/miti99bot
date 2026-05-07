/**
 * @file registry — loads modules listed in env.MODULES, validates their
 * commands, and builds the dispatcher's lookup tables.
 *
 * Design highlights (see plan phase-04):
 *  - Static import map (./index.js) filtered at runtime; preserves tree-shaking.
 *  - Three visibility-partitioned maps (`publicCommands`, `protectedCommands`,
 *    `privateCommands`) PLUS a flat `allCommands` map used by the dispatcher.
 *  - Unified namespace for conflict detection: two commands with the same name
 *    in different modules collide regardless of visibility. Fail loud at load.
 *  - Built once per warm instance (memoized behind `getCurrentRegistry`).
 *  - `resetRegistry()` exists for tests.
 */

import { createSqlStore } from "../db/create-sql-store.js";
import { createStore } from "../db/create-store.js";
import { moduleRegistry as defaultModuleRegistry } from "./index.js";
import { validateCommand } from "./validate-command.js";
import { validateCron } from "./validate-cron.js";

/**
 * @typedef {import("./validate-command.js").ModuleCommand} ModuleCommand
 *
 * @typedef {import("./validate-cron.js").ModuleCron} ModuleCron
 *
 * @typedef {object} BotModule
 * @property {string} name
 * @property {ModuleCommand[]} commands
 * @property {ModuleCron[]} [crons]
 * @property {(ctx: { db: any, sql: any, env: any }) => Promise<void>} [init]
 *
 * @typedef {object} RegistryEntry
 * @property {BotModule} module
 * @property {ModuleCommand} cmd
 * @property {"public"|"protected"|"private"} [visibility]
 *
 * @typedef {object} CronEntry
 * @property {BotModule} module
 * @property {string} schedule
 * @property {string} name
 * @property {ModuleCron["handler"]} handler
 *
 * @typedef {object} Registry
 * @property {Map<string, RegistryEntry>} publicCommands
 * @property {Map<string, RegistryEntry>} protectedCommands
 * @property {Map<string, RegistryEntry>} privateCommands
 * @property {Map<string, RegistryEntry>} allCommands
 * @property {CronEntry[]} crons — flat list of all validated cron entries across modules.
 * @property {BotModule[]} modules — ordered per env.MODULES for /help rendering.
 */

/** @type {Registry | null} */
let currentRegistry = null;

/**
 * Parse env.MODULES → trimmed, deduped array of module names.
 *
 * @param {{ MODULES?: string }} env
 * @returns {string[]}
 */
function parseModulesEnv(env) {
  const raw = env?.MODULES;
  if (typeof raw !== "string" || raw.trim().length === 0) {
    throw new Error("MODULES env var is empty");
  }
  const seen = new Set();
  const out = [];
  for (const piece of raw.split(",")) {
    const name = piece.trim();
    if (name.length === 0) continue;
    if (seen.has(name)) continue;
    seen.add(name);
    out.push(name);
  }
  if (out.length === 0) {
    throw new Error("MODULES env var is empty after trim/dedupe");
  }
  return out;
}

/**
 * Resolve each module name to its default export and validate shape.
 *
 * @param {any} env
 * @param {Record<string, () => Promise<{ default: BotModule }>>} [importMap]
 *   Optional injection for tests. Defaults to the static production map.
 * @returns {Promise<BotModule[]>}
 */
export async function loadModules(env, importMap = defaultModuleRegistry) {
  const names = parseModulesEnv(env);
  const modules = [];

  for (const name of names) {
    const loader = importMap[name];
    if (typeof loader !== "function") {
      throw new Error(`unknown module: ${name}`);
    }

    const loaded = await loader();
    const mod = loaded?.default;
    if (!mod || typeof mod !== "object") {
      throw new Error(`module "${name}" must have a default export`);
    }
    if (mod.name !== name) {
      throw new Error(`module "${name}" default export has mismatched name "${mod.name}"`);
    }
    if (!Array.isArray(mod.commands)) {
      throw new Error(`module "${name}" must export a commands array`);
    }
    for (const cmd of mod.commands) validateCommand(cmd, name);

    // Validate crons if present (optional field).
    if (mod.crons !== undefined) {
      if (!Array.isArray(mod.crons)) {
        throw new Error(`module "${name}" crons must be an array`);
      }
      const cronNames = new Set();
      for (const cron of mod.crons) {
        validateCron(cron, name);
        if (cronNames.has(cron.name)) {
          throw new Error(`module "${name}" has duplicate cron name "${cron.name}"`);
        }
        cronNames.add(cron.name);
      }
    }

    modules.push(mod);
  }

  return modules;
}

/**
 * Build the registry: call each module's init, flatten commands into four maps,
 * detect conflicts, and memoize for later getCurrentRegistry() calls.
 *
 * @param {any} env
 * @param {Record<string, () => Promise<{ default: BotModule }>>} [importMap]
 * @returns {Promise<Registry>}
 */
export async function buildRegistry(env, importMap) {
  const modules = await loadModules(env, importMap);

  /** @type {Map<string, RegistryEntry>} */
  const publicCommands = new Map();
  /** @type {Map<string, RegistryEntry>} */
  const protectedCommands = new Map();
  /** @type {Map<string, RegistryEntry>} */
  const privateCommands = new Map();
  /** @type {Map<string, RegistryEntry>} */
  const allCommands = new Map();
  /** @type {CronEntry[]} */
  const crons = [];

  for (const mod of modules) {
    if (typeof mod.init === "function") {
      try {
        await mod.init({ db: createStore(mod.name, env), sql: createSqlStore(mod.name, env), env });
      } catch (err) {
        throw new Error(
          `module "${mod.name}" init failed: ${err instanceof Error ? err.message : String(err)}`,
          { cause: err },
        );
      }
    }

    for (const cmd of mod.commands) {
      const existing = allCommands.get(cmd.name);
      if (existing) {
        throw new Error(
          `command conflict: /${cmd.name} registered by both "${existing.module.name}" and "${mod.name}"`,
        );
      }
      const entry = { module: mod, cmd, visibility: cmd.visibility };
      allCommands.set(cmd.name, entry);

      if (cmd.visibility === "public") publicCommands.set(cmd.name, entry);
      else if (cmd.visibility === "protected") protectedCommands.set(cmd.name, entry);
      else privateCommands.set(cmd.name, entry);
    }

    // Collect cron entries (validated during loadModules).
    if (Array.isArray(mod.crons)) {
      for (const cron of mod.crons) {
        crons.push({
          module: mod,
          schedule: cron.schedule,
          name: cron.name,
          handler: cron.handler,
        });
      }
    }
  }

  const registry = {
    publicCommands,
    protectedCommands,
    privateCommands,
    allCommands,
    crons,
    modules,
  };
  currentRegistry = registry;
  return registry;
}

/**
 * @returns {Registry}
 * @throws if `buildRegistry` has not been called yet.
 */
export function getCurrentRegistry() {
  if (!currentRegistry) {
    throw new Error("registry not built yet — call buildRegistry(env) first");
  }
  return currentRegistry;
}

/**
 * Test helper — forget the memoized registry so each test starts clean.
 */
export function resetRegistry() {
  currentRegistry = null;
}
