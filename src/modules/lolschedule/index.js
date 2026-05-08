/**
 * @file lolschedule module — LoL esports match schedule via the
 * lolesports.com esports-api (the data feed behind lolesports.com).
 *
 * Commands:
 *   /lolschedule [date] — matches for a specific ICT day (defaults to today).
 *                         Accepts dd-mm-yyyy, dd/mm/yyyy, ddmmyyyy; trailing
 *                         month/year may be omitted.
 *   /lolschedule_today — matches scheduled for the current ICT day, with live/played scores.
 *   /lolschedule_week  — next 7 ICT days, grouped per day → league.
 *
 * Cron:
 *   0 1 * * *  (08:00 ICT) — push today's major-league schedule to every
 *   chat that has opted in via /lolschedule_subscribe.
 *
 * See the module README for data-source rationale and the verification
 * reports under plans/reports/ for historical context.
 */

import {
  handleDailyPushCron,
  handleSchedule,
  handleSubscribe,
  handleToday,
  handleUnsubscribe,
  handleWeek,
} from "./handlers.js";

/** @type {import("../../db/kv-store-interface.js").KVStore | null} */
let db = null;

/** @type {import("../registry.js").BotModule} */
const lolscheduleModule = {
  name: "lolschedule",
  init: async ({ db: store }) => {
    db = store;
  },
  commands: [
    {
      name: "lolschedule",
      visibility: "public",
      description: "LoL matches for a date (dd-mm-yyyy, dd/mm/yyyy, ddmmyyyy; default today)",
      handler: (ctx) => handleSchedule(ctx, db),
    },
    {
      name: "lolschedule_today",
      visibility: "public",
      description: "Today's LoL esports matches (scores if played)",
      handler: (ctx) => handleToday(ctx, db),
    },
    {
      name: "lolschedule_week",
      visibility: "public",
      description: "LoL esports matches for the next 7 days",
      handler: (ctx) => handleWeek(ctx, db),
    },
    {
      name: "lolschedule_subscribe",
      visibility: "public",
      description: "Get the daily LoL schedule digest at 08:00 ICT",
      handler: (ctx) => handleSubscribe(ctx, db),
    },
    {
      name: "lolschedule_unsubscribe",
      visibility: "public",
      description: "Stop receiving the daily LoL schedule digest",
      handler: (ctx) => handleUnsubscribe(ctx, db),
    },
  ],
  crons: [
    {
      schedule: "0 1 * * *",
      name: "daily-push",
      handler: handleDailyPushCron,
    },
  ],
};

export default lolscheduleModule;
