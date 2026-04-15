import jsdoc from "eslint-plugin-jsdoc";

// Cloudflare Workers globals + custom codebase types that no-undefined-types
// must treat as defined (no tsc/jsconfig — plain ESM JS).
const DEFINED_TYPES = [
  // Cloudflare Workers runtime globals
  "KVNamespace",
  "D1Database",
  "D1PreparedStatement",
  // Web globals not always in eslint's scope
  "Request",
  "Response",
  "URL",
  // KV interface types (src/db/kv-store-interface.js)
  "KVStore",
  "KVStorePutOptions",
  "KVStoreListOptions",
  "KVStoreListResult",
  // SQL interface types (src/db/sql-store-interface.js)
  "SqlStore",
  "SqlRunResult",
  // Central typedefs (src/types.js)
  "Env",
  "Module",
  "Command",
  "Cron",
  "ModuleContext",
  "Trade",
  "Portfolio",
  "PortfolioMeta",
  // Module sub-types (registry, validate-*)
  "BotModule",
  "RegistryEntry",
  "Registry",
  "ModuleCommand",
  "ModuleCron",
  "CronEntry",
  // Trading sub-types
  "ResolvedSymbol",
];

export default [
  {
    files: ["src/**/*.js"],
    plugins: { jsdoc },
    rules: {
      "jsdoc/check-alignment": "warn",
      "jsdoc/check-param-names": "error",
      "jsdoc/check-tag-names": "error",
      "jsdoc/check-types": "error",
      "jsdoc/no-undefined-types": ["error", { definedTypes: DEFINED_TYPES }],
      "jsdoc/require-param-type": "error",
      "jsdoc/require-returns-type": "error",
      "jsdoc/valid-types": "error",
    },
  },
];
