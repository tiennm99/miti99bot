/**
 * @file lolschedule module — LoL esports match schedule via the
 * lolesports.com esports-api (the data feed behind lolesports.com).
 *
 * Commands:
 *   /lol_today — matches scheduled for the current ICT day, with live/played scores.
 *   /lol_week  — next 7 ICT days, grouped per day.
 *
 * See the module README for the data-source rationale and the verification
 * reports under plans/reports/ for historical context.
 */

import { handleToday, handleWeek } from "./handlers.js";

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
      name: "lol_today",
      visibility: "public",
      description: "Today's LoL esports matches (scores if played)",
      handler: (ctx) => handleToday(ctx, db),
    },
    {
      name: "lol_week",
      visibility: "public",
      description: "LoL esports matches for the next 7 days",
      handler: (ctx) => handleWeek(ctx, db),
    },
  ],
};

export default lolscheduleModule;
