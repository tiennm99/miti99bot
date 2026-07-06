export const minRenderTimeoutMs = 7000;

/**
 * @typedef {object} AppConfig
 * @property {string} host
 * @property {number} port
 * @property {number} maxConcurrentRenders
 * @property {number} renderTimeoutMs
 * @property {number} maxOptions
 * @property {number} maxOptionChars
 * @property {string | undefined} apiToken
 * @property {boolean} requiresApiToken
 */

/**
 * @param {string | undefined} value
 * @param {number} fallback
 * @returns {number}
 */
const parsePositiveInt = (value, fallback) => {
  if (!value) {
    return fallback;
  }

  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
};

/**
 * @param {NodeJS.ProcessEnv} [env]
 * @returns {AppConfig}
 */
export const loadConfig = (env = process.env) => {
  const apiToken = env.API_TOKEN || undefined;
  const requiresApiToken = env.NODE_ENV === 'production' || env.REQUIRE_API_TOKEN === 'true';

  if (requiresApiToken && !apiToken) {
    throw new Error('API_TOKEN is required when NODE_ENV=production or REQUIRE_API_TOKEN=true');
  }

  return {
    host: env.HOST || '0.0.0.0',
    port: parsePositiveInt(env.PORT, 3000),
    maxConcurrentRenders: parsePositiveInt(env.MAX_CONCURRENT_RENDERS, 1),
    renderTimeoutMs: Math.max(
      minRenderTimeoutMs,
      parsePositiveInt(env.RENDER_TIMEOUT_MS, 15000),
    ),
    maxOptions: parsePositiveInt(env.MAX_OPTIONS, 32),
    maxOptionChars: parsePositiveInt(env.MAX_OPTION_CHARS, 40),
    apiToken,
    requiresApiToken,
  };
};
