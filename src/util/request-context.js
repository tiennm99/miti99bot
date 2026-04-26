/**
 * @file request-context — module-scoped request state shared between
 * the fetch entry point (index.js) and the dispatcher middleware.
 *
 * CF Workers isolates are single-threaded: one request at a time, so a
 * module-scope variable is a safe per-request stash without race conditions.
 *
 * Placing this in a separate module breaks the circular import that would
 * arise from dispatcher.js importing index.js directly.
 */

/**
 * Cold-start metadata captured at the top of each fetch() call.
 * Written by index.js; read by dispatcher timing middleware.
 *
 * @type {{ cold: boolean, isolateAgeMs: number }}
 */
let _lastCold = { cold: false, isolateAgeMs: 0 };

/**
 * Store the cold-start metadata for the current request.
 * Called once per fetch() invocation, before any awaits.
 *
 * @param {{ cold: boolean, isolateAgeMs: number }} value
 */
export function setLastCold(value) {
  _lastCold = value;
}

/**
 * Retrieve the cold-start metadata for the current request.
 * Called by dispatcher timing middleware.
 *
 * @returns {{ cold: boolean, isolateAgeMs: number }}
 */
export function getLastCold() {
  return _lastCold;
}
