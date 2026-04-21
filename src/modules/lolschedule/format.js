/**
 * @file Pure formatters — Telegram HTML output for today / week match lists.
 *
 * Takes lolesports.com schedule events and renders them. All user-influenced
 * substrings are HTML-escaped. Times are shown in ICT (UTC+7).
 */

import { escapeHtml } from "../../util/escape-html.js";

/** @typedef {import("./api-client.js").ScheduleEvent} ScheduleEvent */
/** @typedef {import("./api-client.js").Team} Team */

const TZ_OFFSET_MS = 7 * 60 * 60 * 1000; // ICT = UTC+7

/** Shift a UTC Date by the ICT offset so getUTC* yields ICT components. */
function toIct(date) {
  return new Date(date.getTime() + TZ_OFFSET_MS);
}

function pad(n) {
  return String(n).padStart(2, "0");
}

export function formatIctTime(date) {
  const d = toIct(date);
  return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
}

export function formatIctDayLabel(date) {
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

function ictDayKey(date) {
  const d = toIct(date);
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
}

/** Pick the visible short tag for a team. */
function teamLabel(team) {
  if (!team) return "TBD";
  return team.code || team.name || "TBD";
}

/**
 * Render one event line (no leading newline).
 *
 * @param {ScheduleEvent} event
 * @returns {string} escaped HTML
 */
export function formatEventLine(event) {
  const teams = event?.match?.teams || [];
  const t1Label = escapeHtml(teamLabel(teams[0]));
  const t2Label = escapeHtml(teamLabel(teams[1]));
  const league = escapeHtml(event?.league?.name || "");
  const block = event?.blockName ? ` (${escapeHtml(event.blockName)})` : "";
  const bestOf = event?.match?.strategy?.count;
  const bo = bestOf ? ` · Bo${bestOf}` : "";

  if (event?.state === "completed") {
    const w1 = teams[0]?.result?.gameWins ?? 0;
    const w2 = teams[1]?.result?.gameWins ?? 0;
    const winner1 = teams[0]?.result?.outcome === "win";
    const winner2 = teams[1]?.result?.outcome === "win";
    const l = winner1 ? `<b>${t1Label}</b>` : t1Label;
    const r = winner2 ? `<b>${t2Label}</b>` : t2Label;
    return `✅ ${l} ${w1}–${w2} ${r}${bo} · ${league}${block}`;
  }
  if (event?.state === "inProgress") {
    const w1 = teams[0]?.result?.gameWins ?? 0;
    const w2 = teams[1]?.result?.gameWins ?? 0;
    return `🔴 LIVE ${t1Label} ${w1}–${w2} ${t2Label}${bo} · ${league}${block}`;
  }
  // unstarted or unknown
  const time = formatIctTime(new Date(event.startTime));
  return `🕒 ${time} ${t1Label} vs ${t2Label}${bo} · ${league}${block}`;
}

/**
 * Render today's reply.
 *
 * @param {ScheduleEvent[]} events
 * @param {Date} day — any moment on the target ICT day.
 * @returns {string}
 */
export function renderToday(events, day) {
  const header = `<b>LoL — ${escapeHtml(formatIctDayLabel(day))}</b> (ICT)`;
  if (events.length === 0) return `${header}\nNo matches today.`;
  return `${header}\n${events.map(formatEventLine).join("\n")}`;
}

/**
 * Render week reply — grouped by ICT day.
 *
 * @param {ScheduleEvent[]} events
 * @param {Date} from
 * @param {Date} to
 * @returns {string}
 */
export function renderWeek(events, from, to) {
  const fromLbl = escapeHtml(formatIctDayLabel(from));
  const toLbl = escapeHtml(formatIctDayLabel(new Date(to.getTime() - 1)));
  const header = `<b>LoL — ${fromLbl} → ${toLbl}</b> (ICT)`;
  if (events.length === 0) return `${header}\nNo matches this week.`;

  /** @type {Map<string, { label: string, lines: string[] }>} */
  const groups = new Map();
  for (const event of events) {
    const d = new Date(event.startTime);
    const key = ictDayKey(d);
    let g = groups.get(key);
    if (!g) {
      g = { label: formatIctDayLabel(d), lines: [] };
      groups.set(key, g);
    }
    g.lines.push(formatEventLine(event));
  }

  const sections = [];
  for (const key of [...groups.keys()].sort()) {
    const g = groups.get(key);
    sections.push(`<b>${escapeHtml(g.label)}</b>\n${g.lines.join("\n")}`);
  }
  return `${header}\n\n${sections.join("\n\n")}`;
}
