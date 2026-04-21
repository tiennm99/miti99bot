/**
 * @file Pure formatters — Telegram HTML output for today / week match lists.
 *
 * Takes lolesports.com schedule events and renders them, grouped by league
 * (today) and by day → league (week). All user-influenced substrings are
 * HTML-escaped. Times are shown in ICT (UTC+7).
 */

import { escapeHtml } from "../../util/escape-html.js";

/** @typedef {import("./api-client.js").ScheduleEvent} ScheduleEvent */
/** @typedef {import("./api-client.js").Team} Team */

const TZ_OFFSET_MS = 7 * 60 * 60 * 1000; // ICT = UTC+7

/** Ordering for league sections — most prestigious tournaments first. */
const LEAGUE_ORDER = [
  "worlds",
  "msi",
  "first_stand",
  "lck",
  "lpl",
  "lec",
  "lcs",
  "lcp",
  "cblol-brazil",
  "emea_masters",
];

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
 * Render one event line (no leading newline). The league name is omitted
 * because events render under a league header.
 *
 * @param {ScheduleEvent} event
 * @returns {string} escaped HTML
 */
export function formatEventLine(event) {
  const teams = event?.match?.teams || [];
  const t1Label = escapeHtml(teamLabel(teams[0]));
  const t2Label = escapeHtml(teamLabel(teams[1]));
  const block = event?.blockName ? ` (${escapeHtml(event.blockName)})` : "";
  const bestOf = event?.match?.strategy?.count;
  const bo = bestOf ? ` · Bo${bestOf}` : "";

  if (event?.state === "completed") {
    const w1 = teams[0]?.result?.gameWins ?? 0;
    const w2 = teams[1]?.result?.gameWins ?? 0;
    const l = teams[0]?.result?.outcome === "win" ? `<b>${t1Label}</b>` : t1Label;
    const r = teams[1]?.result?.outcome === "win" ? `<b>${t2Label}</b>` : t2Label;
    return `✅ ${l} ${w1}–${w2} ${r}${bo}${block}`;
  }
  if (event?.state === "inProgress") {
    const w1 = teams[0]?.result?.gameWins ?? 0;
    const w2 = teams[1]?.result?.gameWins ?? 0;
    return `🔴 LIVE ${t1Label} ${w1}–${w2} ${t2Label}${bo}${block}`;
  }
  const time = formatIctTime(new Date(event.startTime));
  return `🕒 ${time} ${t1Label} vs ${t2Label}${bo}${block}`;
}

/**
 * Group events by league slug, preserving LEAGUE_ORDER for known leagues and
 * falling back to alphabetical for anything else.
 *
 * @param {ScheduleEvent[]} events
 * @returns {Array<{ slug: string, name: string, events: ScheduleEvent[] }>}
 */
function groupByLeague(events) {
  /** @type {Map<string, { slug: string, name: string, events: ScheduleEvent[] }>} */
  const bySlug = new Map();
  for (const event of events) {
    const slug = event?.league?.slug || "unknown";
    const name = event?.league?.name || slug;
    let g = bySlug.get(slug);
    if (!g) {
      g = { slug, name, events: [] };
      bySlug.set(slug, g);
    }
    g.events.push(event);
  }
  const known = LEAGUE_ORDER.filter((slug) => bySlug.has(slug)).map((slug) => bySlug.get(slug));
  const unknown = [...bySlug.values()]
    .filter((g) => !LEAGUE_ORDER.includes(g.slug))
    .sort((a, b) => a.name.localeCompare(b.name));
  return [...known, ...unknown];
}

/** Render a league section (header + lines). */
function renderLeagueSection(group) {
  const lines = group.events.map((e) => formatEventLine(e));
  return `<b>${escapeHtml(group.name)}</b>\n${lines.join("\n")}`;
}

/**
 * Render today's reply — grouped by league.
 *
 * @param {ScheduleEvent[]} events
 * @param {Date} day — any moment on the target ICT day.
 * @returns {string}
 */
export function renderToday(events, day) {
  const header = `<b>LoL — ${escapeHtml(formatIctDayLabel(day))}</b> (ICT)`;
  if (events.length === 0) return `${header}\nNo matches today.`;
  const sections = groupByLeague(events).map(renderLeagueSection);
  return `${header}\n\n${sections.join("\n\n")}`;
}

/**
 * Render week reply — grouped by league → ICT day. The league ordering is
 * determined by LEAGUE_ORDER; within each league, days appear chronologically
 * and each day section lists its matches.
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

  const leagueBlocks = groupByLeague(events).map((league) => {
    /** @type {Map<string, { label: string, lines: string[] }>} */
    const daysInLeague = new Map();
    for (const event of league.events) {
      const d = new Date(event.startTime);
      const key = ictDayKey(d);
      let g = daysInLeague.get(key);
      if (!g) {
        g = { label: formatIctDayLabel(d), lines: [] };
        daysInLeague.set(key, g);
      }
      g.lines.push(formatEventLine(event));
    }
    const daySections = [...daysInLeague.keys()].sort().map((key) => {
      const day = daysInLeague.get(key);
      return `<i>${escapeHtml(day.label)}</i>\n${day.lines.join("\n")}`;
    });
    return `<b>${escapeHtml(league.name)}</b>\n${daySections.join("\n")}`;
  });

  return `${header}\n\n${leagueBlocks.join("\n\n")}`;
}
