/**
 * @file moduleRegistry — static import map of every buildable module.
 *
 * wrangler bundles statically — dynamic `import(variablePath)` defeats
 * tree-shaking and can fail at bundle time. So we enumerate every module here
 * as a lazy loader, and {@link loadModules} filters the list at runtime
 * against `env.MODULES` (comma-separated). Adding a new module is a two-step
 * edit: create the folder, then add one line here.
 */

export const moduleRegistry = {
  util: () => import("./util/index.js"),
  wordle: () => import("./wordle/index.js"),
  loldle: () => import("./loldle/index.js"),
  misc: () => import("./misc/index.js"),
  trading: () => import("./trading/index.js"),
};
