/**
 * @file Pure formatters — Telegram HTML output for today / week match lists.
 *
 * All user-influenced substrings are escaped; match rows come from the public
 * wiki so we treat them as untrusted. Times are rendered in ICT (UTC+7) since
 * the bot's primary audience is VN; feel free to branch by chat locale later.
 */

import { escapeHtml } from "../../util/escape-html.js";

/** @typedef {import("./api-client.js").MatchRow} MatchRow */

const TZ_OFFSET_MS = 7 * 60 * 60 * 1000; // ICT = UTC+7

/** Parse Leaguepedia's `YYYY-MM-DD HH:MM:SS` UTC literal into a Date. */
export function parseUtc(literal) {
  return new Date(`${literal.replace(" ", "T")}Z`);
}

/** Shift a UTC date by the ICT offset so getUTC* methods yield ICT components. */
function toIct(date) {
  return new Date(date.getTime() + TZ_OFFSET_MS);
}

/** Format ICT time as `HH:MM`. */
function formatIctTime(date) {
  const d = toIct(date);
  const pad = (n) => String(n).padStart(2, "0");
  return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
}

/** Format ICT date as `Mon Apr 21` (weekday + month + day). */
function formatIctDayLabel(date) {
  const d = toIct(date);
  const weekdays = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const months = [
    "Jan",
    "Feb",
    "Mar",
    "Apr",
    "May",
    "Jun",
    "Jul",
    "Aug",
    "Sep",
    "Oct",
    "Nov",
    "Dec",
  ];
  return `${weekdays[d.getUTCDay()]} ${months[d.getUTCMonth()]} ${d.getUTCDate()}`;
}

/** ICT calendar-day key `YYYY-MM-DD` (used for grouping). */
function ictDayKey(date) {
  const d = toIct(date);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
}

/** Coerce Leaguepedia's string score into a number, or null. */
function parseScore(s) {
  if (s == null || s === "") return null;
  const n = Number(s);
  return Number.isFinite(n) ? n : null;
}

/**
 * Classify a match row's display state.
 *
 * @param {MatchRow} row
 * @param {number} nowMs — injected for testability.
 * @returns {"played"|"live"|"scheduled"}
 */
export function classifyMatch(row, nowMs = Date.now()) {
  const startMs = parseUtc(row.DateTime).getTime();
  const winner = String(row.Winner ?? "");
  if (winner === "1" || winner === "2") return "played";
  const s1 = parseScore(row.S1);
  const s2 = parseScore(row.S2);
  const hasScore = (s1 ?? 0) + (s2 ?? 0) > 0;
  if (startMs <= nowMs && hasScore) return "live";
  return "scheduled";
}

/**
 * Render one match line (no leading newline).
 *
 * @param {MatchRow} row
 * @param {number} [nowMs]
 * @returns {string} HTML — already escaped
 */
export function formatMatchLine(row, nowMs = Date.now()) {
  const t1 = escapeHtml(row.T1 || "TBD");
  const t2 = escapeHtml(row.T2 || "TBD");
  const tournament = escapeHtml(row.Tournament || "");
  const bo = row.BO ? ` · Bo${escapeHtml(row.BO)}` : "";
  const state = classifyMatch(row, nowMs);

  if (state === "played") {
    const s1 = parseScore(row.S1) ?? 0;
    const s2 = parseScore(row.S2) ?? 0;
    const w1 = String(row.Winner) === "1" ? "<b>" : "";
    const w1c = w1 ? "</b>" : "";
    const w2 = String(row.Winner) === "2" ? "<b>" : "";
    const w2c = w2 ? "</b>" : "";
    return `✅ ${w1}${t1}${w1c} ${s1}–${s2} ${w2}${t2}${w2c}${bo} · ${tournament}`;
  }
  if (state === "live") {
    const s1 = parseScore(row.S1) ?? 0;
    const s2 = parseScore(row.S2) ?? 0;
    return `🔴 LIVE ${t1} ${s1}–${s2} ${t2}${bo} · ${tournament}`;
  }
  const time = formatIctTime(parseUtc(row.DateTime));
  return `🕒 ${time} ${t1} vs ${t2}${bo} · ${tournament}`;
}

/**
 * Render the "today" command reply.
 *
 * @param {MatchRow[]} rows
 * @param {Date} day — any moment on the target ICT day.
 * @param {number} [nowMs]
 * @returns {string}
 */
export function renderToday(rows, day, nowMs = Date.now()) {
  const header = `<b>LoL — ${escapeHtml(formatIctDayLabel(day))}</b> (ICT)`;
  if (rows.length === 0) return `${header}\nNo matches today.`;
  return `${header}\n${rows.map((r) => formatMatchLine(r, nowMs)).join("\n")}`;
}

/**
 * Render the "week" command reply — matches grouped by ICT day.
 *
 * @param {MatchRow[]} rows
 * @param {Date} from
 * @param {Date} to
 * @param {number} [nowMs]
 * @returns {string}
 */
export function renderWeek(rows, from, to, nowMs = Date.now()) {
  const fromLbl = escapeHtml(formatIctDayLabel(from));
  // `to` is exclusive; subtract 1 ms for a friendlier "through" label.
  const toLbl = escapeHtml(formatIctDayLabel(new Date(to.getTime() - 1)));
  const header = `<b>LoL — ${fromLbl} → ${toLbl}</b> (ICT)`;
  if (rows.length === 0) return `${header}\nNo matches this week.`;

  /** @type {Map<string, { label: string, lines: string[] }>} */
  const groups = new Map();
  for (const row of rows) {
    const d = parseUtc(row.DateTime);
    const key = ictDayKey(d);
    let g = groups.get(key);
    if (!g) {
      g = { label: formatIctDayLabel(d), lines: [] };
      groups.set(key, g);
    }
    g.lines.push(formatMatchLine(row, nowMs));
  }

  const sections = [];
  const sortedKeys = [...groups.keys()].sort();
  for (const key of sortedKeys) {
    const g = groups.get(key);
    sections.push(`<b>${escapeHtml(g.label)}</b>\n${g.lines.join("\n")}`);
  }
  return `${header}\n\n${sections.join("\n\n")}`;
}
