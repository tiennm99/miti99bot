/**
 * @file Leaguepedia cargoquery client for LoL esports match schedule.
 *
 * Uses the MediaWiki Cargo extension on lol.fandom.com. No auth. We identify
 * ourselves with a contact UA (Fandom policy) and cache via both the Worker
 * edge cache (`cf.cacheTtl`) and the module's KVStore for cross-edge reuse.
 *
 * @see plans/reports/researcher-260421-0845-leaguepedia-api-verification.md
 */

const API_URL = "https://lol.fandom.com/api.php";
const USER_AGENT = "miti99bot/0.1 (https://t.me/miti99bot; minhtienit99@gmail.com)";

/** Default KV cache windows — short enough to catch score updates mid-day. */
const CACHE_TTL_TODAY_SEC = 60;
const CACHE_TTL_WEEK_SEC = 300;

/**
 * @typedef {object} MatchRow
 * @property {string} DateTime — "YYYY-MM-DD HH:MM:SS" UTC (Leaguepedia convention).
 * @property {string} T1
 * @property {string} T2
 * @property {string|null} S1 — team-1 score, may be empty string before match.
 * @property {string|null} S2
 * @property {string|null} Winner — "1" | "2" | "" when unplayed.
 * @property {string} Tournament
 * @property {string|null} BO — best-of count as string.
 * @property {string} OP — OverviewPage (wiki slug) for deep-link.
 */

/**
 * Low-level cargoquery POST. Uses POST to avoid edge WAF stripping of `>=`/`<`
 * in the query string and to stay under URL-length limits for long where clauses.
 *
 * @param {Record<string, string>} params
 * @returns {Promise<object[]>} array of row `title` objects
 */
async function cargoQuery(params) {
  const body = new URLSearchParams({
    action: "cargoquery",
    format: "json",
    ...params,
  });
  const res = await fetch(API_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      "User-Agent": USER_AGENT,
      Accept: "application/json",
    },
    body,
    cf: { cacheTtl: 30, cacheEverything: true },
  });
  const text = await res.text();
  if (!res.ok) {
    console.log(
      JSON.stringify({ msg: "lolschedule_fetch", status: res.status, body: text.slice(0, 500) }),
    );
    throw new Error(`Leaguepedia API HTTP ${res.status}`);
  }
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    console.log(
      JSON.stringify({ msg: "lolschedule_parse_fail", body: text.slice(0, 500) }),
    );
    throw new Error("Leaguepedia non-JSON response");
  }
  if (json?.error) {
    console.log(
      JSON.stringify({
        msg: "lolschedule_api_error",
        code: json.error.code,
        info: json.error.info,
      }),
    );
    throw new Error(`Leaguepedia error: ${json.error.info || json.error.code}`);
  }
  const rows = (json?.cargoquery || []).map((r) => r.title);
  console.log(JSON.stringify({ msg: "lolschedule_fetch_ok", rows: rows.length }));
  return rows;
}

/** Format a JS Date as Leaguepedia's UTC literal: `YYYY-MM-DD HH:MM:SS`. */
function toUtcLiteral(date) {
  const pad = (n) => String(n).padStart(2, "0");
  return (
    `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ` +
    `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`
  );
}

/**
 * Fetch matches with DateTime_UTC in [from, to).
 *
 * @param {Date} from
 * @param {Date} to
 * @returns {Promise<MatchRow[]>}
 */
export async function fetchMatchesInRange(from, to) {
  const fromLit = toUtcLiteral(from);
  const toLit = toUtcLiteral(to);
  const rows = await cargoQuery({
    tables: "MatchSchedule=MS",
    fields:
      "MS.DateTime_UTC=DateTime," +
      "MS.Team1=T1,MS.Team2=T2," +
      "MS.Team1Score=S1,MS.Team2Score=S2," +
      "MS.Winner=Winner,MS.Tournament=Tournament," +
      "MS.BestOf=BO,MS.OverviewPage=OP",
    where: `MS.DateTime_UTC >= "${fromLit}" AND MS.DateTime_UTC < "${toLit}"`,
    order_by: "MS.DateTime_UTC ASC",
    limit: "100",
  });
  return /** @type {MatchRow[]} */ (rows);
}

/**
 * Cache-first match lookup keyed by date range. Returns cached rows without
 * refetch within TTL; on fetch failure, returns stale cache if available.
 *
 * @param {import("../../db/kv-store-interface.js").KVStore} db
 * @param {Date} from
 * @param {Date} to
 * @param {number} ttlSec
 * @returns {Promise<MatchRow[]>}
 */
export async function getCachedMatches(db, from, to, ttlSec) {
  const key = `matches:${from.toISOString()}:${to.toISOString()}`;
  const cached = await db.getJSON(key);
  if (cached?.ts && Date.now() - cached.ts < ttlSec * 1000) {
    return cached.rows;
  }
  try {
    const rows = await fetchMatchesInRange(from, to);
    try {
      await db.putJSON(key, { ts: Date.now(), rows }, { expirationTtl: ttlSec * 4 });
    } catch (err) {
      console.warn("lolschedule: KV putJSON failed", String(err));
    }
    return rows;
  } catch (err) {
    if (cached?.rows) return cached.rows; // stale fallback
    throw err;
  }
}

export { CACHE_TTL_TODAY_SEC, CACHE_TTL_WEEK_SEC };
