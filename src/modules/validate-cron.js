/**
 * @file validate-cron — validates module-registered cron entries.
 *
 * Cron entry contract:
 *   - name: ^[a-z0-9_-]{1,32}$ — unique within the module (checked by registry)
 *   - schedule: non-empty string matching a cron-ish pattern
 *     (5 or 6 fields separated by spaces, e.g. "0 1 * * *")
 *   - handler: function
 *
 * All errors include module name + cron name for debuggability.
 */

export const CRON_NAME_RE = /^[a-z0-9_-]{1,32}$/;

/**
 * Very loose cron expression check: 5 or 6 space-separated tokens.
 * Cloudflare Workers validates the real expression at deploy time;
 * we just catch obvious mistakes (empty string, random garbage).
 */
export const CRON_SCHEDULE_RE = /^\S+(\s+\S+){4,5}$/;

/**
 * @typedef {object} ModuleCron
 * @property {string} name — unique identifier within the module.
 * @property {string} schedule — cron expression, e.g. "0 1 * * *".
 * @property {(event: any, ctx: { db: any, sql: any, env: any }) => Promise<void>|void} handler
 */

/**
 * Throws on any contract violation. Called once per cron entry at registry build.
 *
 * @param {any} cron
 * @param {string} moduleName — for error messages.
 */
export function validateCron(cron, moduleName) {
  const prefix = `module "${moduleName}" cron`;

  if (!cron || typeof cron !== "object") {
    throw new Error(`${prefix}: cron entry is not an object`);
  }

  // name
  if (typeof cron.name !== "string") {
    throw new Error(`${prefix}: name must be a string`);
  }
  if (!CRON_NAME_RE.test(cron.name)) {
    throw new Error(
      `${prefix} "${cron.name}": name must match ${CRON_NAME_RE} (lowercase letters, digits, underscore, hyphen; 1–32 chars)`,
    );
  }

  // schedule
  if (typeof cron.schedule !== "string" || cron.schedule.trim().length === 0) {
    throw new Error(`${prefix} "${cron.name}": schedule must be a non-empty string`);
  }
  if (!CRON_SCHEDULE_RE.test(cron.schedule.trim())) {
    throw new Error(
      `${prefix} "${cron.name}": schedule must be a valid cron expression (5 or 6 space-separated fields), got "${cron.schedule}"`,
    );
  }

  // handler
  if (typeof cron.handler !== "function") {
    throw new Error(`${prefix} "${cron.name}": handler must be a function`);
  }
}
