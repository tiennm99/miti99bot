/**
 * @file Parse a user-supplied date param for /lolschedule.
 *
 * Supported formats: dd-mm-yyyy, dd/mm/yyyy, ddmmyyyy. Trailing components
 * may be omitted (year, then month) — falling back to the current ICT day.
 * Empty input → today. Day is mandatory; month/year without day is rejected.
 */

const ICT_OFFSET_MS = 7 * 60 * 60 * 1000;

const FORMAT_HINT = "Use dd-mm-yyyy, dd/mm/yyyy, or ddmmyyyy.";

function toIct(date) {
  return new Date(date.getTime() + ICT_OFFSET_MS);
}

function ictDayStartOf(now) {
  const shifted = toIct(now);
  shifted.setUTCHours(0, 0, 0, 0);
  return new Date(shifted.getTime() - ICT_OFFSET_MS);
}

/**
 * Split the raw input into [dd, mm?, yyyy?] string parts, or return an error.
 *
 * @param {string} trimmed
 * @returns {{ ok: true, parts: string[] } | { ok: false, error: string }}
 */
function splitParts(trimmed) {
  if (trimmed.includes("-") || trimmed.includes("/")) {
    const parts = trimmed.split(/[-/]/);
    if (parts.length < 1 || parts.length > 3) {
      return { ok: false, error: `Invalid date "${trimmed}". ${FORMAT_HINT}` };
    }
    if (parts.some((p) => p === "" || !/^\d+$/.test(p))) {
      return { ok: false, error: `Invalid date "${trimmed}". ${FORMAT_HINT}` };
    }
    return { ok: true, parts };
  }

  if (!/^\d+$/.test(trimmed)) {
    return { ok: false, error: `Invalid date "${trimmed}". ${FORMAT_HINT}` };
  }
  if (trimmed.length === 1 || trimmed.length === 2) {
    return { ok: true, parts: [trimmed] };
  }
  if (trimmed.length === 4) {
    return { ok: true, parts: [trimmed.slice(0, 2), trimmed.slice(2)] };
  }
  if (trimmed.length === 8) {
    return {
      ok: true,
      parts: [trimmed.slice(0, 2), trimmed.slice(2, 4), trimmed.slice(4)],
    };
  }
  return { ok: false, error: `Invalid date "${trimmed}". ${FORMAT_HINT}` };
}

/**
 * Parse a user date param into the start of the requested ICT day.
 *
 * @param {string|undefined|null} input
 * @param {Date} [now]
 * @returns {{ ok: true, date: Date } | { ok: false, error: string }}
 */
export function parseScheduleDate(input, now = new Date()) {
  const trimmed = (input ?? "").trim();
  if (!trimmed) return { ok: true, date: ictDayStartOf(now) };

  const split = splitParts(trimmed);
  if (!split.ok) return split;

  const ictNow = toIct(now);
  const day = Number(split.parts[0]);
  const month = split.parts.length >= 2 ? Number(split.parts[1]) : ictNow.getUTCMonth() + 1;
  const year = split.parts.length >= 3 ? Number(split.parts[2]) : ictNow.getUTCFullYear();

  if (!Number.isInteger(day) || day < 1 || day > 31) {
    return { ok: false, error: `Invalid day "${split.parts[0]}" — must be 1–31.` };
  }
  if (!Number.isInteger(month) || month < 1 || month > 12) {
    return { ok: false, error: `Invalid month "${split.parts[1]}" — must be 1–12.` };
  }
  if (!Number.isInteger(year) || year < 1970 || year > 2100) {
    return { ok: false, error: `Invalid year "${split.parts[2]}".` };
  }

  // ICT midnight of requested day, expressed as a UTC instant.
  const utcMs = Date.UTC(year, month - 1, day, 0, 0, 0) - ICT_OFFSET_MS;
  const date = new Date(utcMs);

  // Reject impossible dates like 31-04 or 29-02 in non-leap years.
  const verified = toIct(date);
  if (
    verified.getUTCFullYear() !== year ||
    verified.getUTCMonth() + 1 !== month ||
    verified.getUTCDate() !== day
  ) {
    return { ok: false, error: `Invalid date — ${day}/${month}/${year} does not exist.` };
  }

  return { ok: true, date };
}
