/**
 * @file lolschedule module — LoL esports match schedule via the
 * lolesports.com esports-api (the data feed behind lolesports.com).
 *
 * Commands:
 *   /lolschedule_today — matches scheduled for the current ICT day, with live/played scores.
 *   /lolschedule_week  — next 7 ICT days, grouped per day → league.
 *
 * Cron:
 *   0 1 * * *  (08:00 ICT) — push today's major-league schedule to
 *   LOLSCHEDULE_CHAT_ID when configured.
 *
 * See the module README for data-source rationale and the verification
 * reports under plans/reports/ for historical context.
 */

import { handleDailyPushCron, handleToday, handleWeek } from "./handlers.js";

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
