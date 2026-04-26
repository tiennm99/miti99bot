/**
 * @file timing — per-request command timing for soak analysis.
 * Logs structured JSON via console.log; CF Observability captures it.
 *
 * Usage:
 *   const t = startTiming("/wordle");
 *   t.mark("mongo-read");
 *   t.end({ cold: true });
 *   // logs: {"event":"cmd_timing","cmd":"/wordle","total":45,"cold":true,"marks":[...]}
 */

/**
 * Start a timing context for a command.
 *
 * @param {string} cmd - command name, e.g. "/wordle"
 * @returns {{ mark: (label: string) => void, end: (extra?: object) => void }}
 */
export function startTiming(cmd) {
  const t0 = Date.now();
  const marks = []; // declared at top — fixes the snippet bug from the plan

  return {
    /**
     * Record a named checkpoint relative to the start.
     *
     * @param {string} label
     */
    mark(label) {
      marks.push({ label, dt: Date.now() - t0 });
    },

    /**
     * Finalize and emit a structured timing log entry.
     *
     * @param {object} [extra={}] - additional key/value pairs merged into the log line
     */
    end(extra = {}) {
      const total = Date.now() - t0;
      console.log(JSON.stringify({ event: "cmd_timing", cmd, total, ...extra, marks }));
    },
  };
}

// ── Cold-start detection ─────────────────────────────────────────────────────
// CF Worker isolates are single-threaded; one isolate handles one request at a
// time. The first request in a fresh isolate is "cold". We use a boolean flag
// (not isolate_age_ms < 200ms) because Mongo connect itself takes ~1500ms,
// making age-based classification unreliable (code-reviewer #11).

let _isFirst = true;
const _isolateBorn = Date.now();

/**
 * Returns cold-start metadata for the current request.
 *
 * First call in an isolate returns `{ cold: true, isolateAgeMs: ~0 }`.
 * Subsequent calls return `{ cold: false, isolateAgeMs: <ms-since-boot> }`.
 *
 * Call exactly once per incoming request (at the top of the fetch handler).
 *
 * @returns {{ cold: boolean, isolateAgeMs: number }}
 */
export function takeColdFlag() {
  const cold = _isFirst;
  _isFirst = false;
  return { cold, isolateAgeMs: Date.now() - _isolateBorn };
}
