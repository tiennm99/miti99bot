export const minRenderTimeoutMs = 7000;

/**
 * @typedef {object} AppConfig
 * @property {string} host
 * @property {number} port
 * @property {number} maxConcurrentRenders
 * @property {number} renderTimeoutMs
 * @property {number} maxOptions
 * @property {number} maxOptionChars
 */

/**
 * Reads a required positive integer setting, recording a problem instead of
 * falling back: compose.yml owns every default, so a missing value is a
 * deployment mistake to report, not to paper over.
 *
 * @param {NodeJS.ProcessEnv} env
 * @param {string} name
 * @param {string[]} problems
 * @returns {number}
 */
const requirePositiveInt = (env, name, problems) => {
  const raw = env[name]?.trim();
  if (!raw) {
    problems.push(`${name} is required`);
    return 0;
  }
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    problems.push(`${name} must be a positive integer, got "${raw}"`);
    return 0;
  }
  return parsed;
};

/**
 * Loads the renderer settings. Every RENDERER_* variable is required; the
 * deployment (compose.yml, or .env for local runs) supplies the values.
 *
 * @param {NodeJS.ProcessEnv} [env]
 * @returns {AppConfig}
 */
export const loadConfig = (env = process.env) => {
  /** @type {string[]} */
  const problems = [];
  const host = env.RENDERER_HOST?.trim() ?? '';
  if (!host) {
    problems.push('RENDERER_HOST is required');
  }
  const config = {
    host,
    port: requirePositiveInt(env, 'RENDERER_PORT', problems),
    maxConcurrentRenders: requirePositiveInt(env, 'RENDERER_MAX_CONCURRENT_RENDERS', problems),
    // Remotion's browser timeout cannot go below 7000ms, so lower values are
    // raised to that floor rather than rejected.
    renderTimeoutMs: Math.max(
      minRenderTimeoutMs,
      requirePositiveInt(env, 'RENDERER_RENDER_TIMEOUT_MS', problems),
    ),
    maxOptions: requirePositiveInt(env, 'RENDERER_MAX_OPTIONS', problems),
    maxOptionChars: requirePositiveInt(env, 'RENDERER_MAX_OPTION_CHARS', problems),
  };
  if (problems.length > 0) {
    throw new Error(`invalid renderer configuration: ${problems.join('; ')}`);
  }
  return config;
};
