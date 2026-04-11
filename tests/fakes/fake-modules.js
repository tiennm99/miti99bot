/**
 * @file fake-modules — fixture modules + a helper that builds an import-map
 * shape compatible with `loadModules(env, importMap)`.
 *
 * Using dependency injection (instead of `vi.mock`) sidesteps path-resolution
 * flakiness on Windows and keeps tests fully deterministic.
 */

/**
 * @param {Record<string, import("../../src/modules/registry.js").BotModule>} modules
 */
export function makeFakeImportMap(modules) {
  /** @type {Record<string, () => Promise<{default: any}>>} */
  const map = {};
  for (const [name, mod] of Object.entries(modules)) {
    map[name] = async () => ({ default: mod });
  }
  return map;
}

export const noopHandler = async () => {};

export function makeCommand(name, visibility, description = "fixture command") {
  return { name, visibility, description, handler: noopHandler };
}

export function makeModule(name, commands, init) {
  return init ? { name, commands, init } : { name, commands };
}
