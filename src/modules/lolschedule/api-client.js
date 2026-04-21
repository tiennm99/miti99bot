/**
 * @file lolesports.com esports-api client.
 *
 * Endpoint: https://esports-api.lolesports.com/persisted/gw/getSchedule
 * Auth: `x-api-key` header — the public key embedded in lolesports.com's own
 *   web client. No registration. No practical rate limit (the same key serves
 *   the live site). If Riot ever rotates it, lift the new value from their
 *   public JS bundle.
 *
 * We cache responses in KV keyed by league filter so concurrent user requests
 * collapse to one upstream hit within the TTL window.
 */

const API_URL = "https://esports-api.lolesports.com/persisted/gw/getSchedule";
const API_KEY = "0TvQnueqKa5mxJntVWt0w4LpLfEkrV1Ta8rQBb9Z";
const USER_AGENT = "miti99bot/0.1 (https://t.me/miti99bot)";

/** Short cache — schedule data changes minute-by-minute during live events. */
const CACHE_TTL_SEC = 120;
/** Stale fallback ceiling for resilience during upstream hiccups. */
const STALE_MAX_AGE_SEC = 60 * 60;

/**
 * @typedef {object} Team
 * @property {string} name
 * @property {string} code — short league tag, e.g. "T1", "GEN".
 * @property {string} [image]
 * @property {{ outcome: "win"|"loss", gameWins: number }} [result]
 * @property {{ wins: number, losses: number }} [record]
 */

/**
 * @typedef {object} ScheduleEvent
 * @property {string} startTime — ISO 8601 UTC.
 * @property {"unstarted"|"inProgress"|"completed"} state
 * @property {string} [blockName] — e.g. "Week 4".
 * @property {{ name: string, slug: string, image?: string }} league
 * @property {{
 *   id: string,
 *   teams: Team[],
 *   strategy: { type: string, count: number }
 * }} match
 */

/**
 * Fetch one page of schedule events. Returns events + pagination tokens.
 *
 * @param {object} [opts]
 * @param {string} [opts.pageToken] — `newer`/`older` cursor from a previous call.
 * @param {string} [opts.leagueId] — optional comma-separated league IDs.
 * @returns {Promise<{ events: ScheduleEvent[], olderToken?: string, newerToken?: string }>}
 */
export async function fetchSchedulePage({ pageToken, leagueId } = {}) {
  const url = new URL(API_URL);
  url.searchParams.set("hl", "en-US");
  if (pageToken) url.searchParams.set("pageToken", pageToken);
  if (leagueId) url.searchParams.set("leagueId", leagueId);

  const res = await fetch(url.toString(), {
    headers: {
      "x-api-key": API_KEY,
      "User-Agent": USER_AGENT,
      Accept: "application/json",
    },
    cf: { cacheTtl: 60, cacheEverything: true },
  });
  const text = await res.text();
  if (!res.ok) {
    console.log(
      JSON.stringify({ msg: "lolschedule_fetch", status: res.status, body: text.slice(0, 500) }),
    );
    throw new Error(`lolesports API HTTP ${res.status}`);
  }
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    throw new Error("lolesports non-JSON response");
  }
  const schedule = json?.data?.schedule;
  const events = Array.isArray(schedule?.events) ? schedule.events : [];
  const filtered = events.filter((e) => e?.type !== "show"); // drop pre/post shows
  return {
    events: /** @type {ScheduleEvent[]} */ (filtered),
    olderToken: schedule?.pages?.older,
    newerToken: schedule?.pages?.newer,
  };
}

/**
 * Fetch enough pages forward in time to cover the given UTC window.
 * Default page returns ~20 events; week view usually needs 1 extra page.
 *
 * @param {Date} from
 * @param {Date} to
 * @param {number} [maxPages] — hard cap to keep it bounded.
 * @returns {Promise<ScheduleEvent[]>}
 */
export async function fetchEventsInRange(from, to, maxPages = 3) {
  const fromMs = from.getTime();
  const toMs = to.getTime();
  /** @type {ScheduleEvent[]} */
  const collected = [];
  let pageToken;
  for (let i = 0; i < maxPages; i++) {
    const { events, newerToken } = await fetchSchedulePage({ pageToken });
    collected.push(...events);
    // If the latest event in the page is already past our window end, stop.
    const lastMs = events.length ? Date.parse(events[events.length - 1].startTime) : null;
    if (lastMs !== null && lastMs >= toMs) break;
    if (!newerToken) break;
    pageToken = newerToken;
  }
  return collected.filter((e) => {
    const t = Date.parse(e.startTime);
    return t >= fromMs && t < toMs;
  });
}

/** Build the KV cache key for a date range. */
function cacheKey(from, to) {
  return `matches:${from.toISOString()}:${to.toISOString()}`;
}

/**
 * Cache-first lookup. Returns fresh cache within TTL, else tries upstream,
 * else returns stale cache up to {@link STALE_MAX_AGE_SEC}, else throws.
 *
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {Date} from
 * @param {Date} to
 * @returns {Promise<ScheduleEvent[]>}
 */
export async function getEventsCached(db, from, to) {
  const key = cacheKey(from, to);
  const cached = await db.getJSON(key);
  if (cached?.ts && Date.now() - cached.ts < CACHE_TTL_SEC * 1000) {
    return cached.events;
  }
  try {
    const events = await fetchEventsInRange(from, to);
    try {
      await db.putJSON(key, { ts: Date.now(), events }, { expirationTtl: STALE_MAX_AGE_SEC });
    } catch (err) {
      console.log(JSON.stringify({ msg: "lolschedule_kv_put_fail", err: String(err) }));
    }
    return events;
  } catch (err) {
    if (cached?.events && cached?.ts && Date.now() - cached.ts < STALE_MAX_AGE_SEC * 1000) {
      console.log(JSON.stringify({ msg: "lolschedule_stale_fallback", err: String(err) }));
      return cached.events;
    }
    throw err;
  }
}

export { CACHE_TTL_SEC, STALE_MAX_AGE_SEC };
