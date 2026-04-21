import { describe, expect, it } from "vitest";
import {
  formatEventLine,
  formatIctDayLabel,
  formatIctTime,
  renderToday,
  renderWeek,
} from "../../../src/modules/lolschedule/format.js";

/** @typedef {import("../../../src/modules/lolschedule/api-client.js").ScheduleEvent} ScheduleEvent */

function evt(overrides = {}) {
  return /** @type {ScheduleEvent} */ ({
    startTime: "2026-04-21T09:00:00Z",
    state: "unstarted",
    blockName: "Week 4",
    league: { name: "LCK", slug: "lck" },
    match: {
      id: "m1",
      teams: [
        { name: "T1", code: "T1" },
        { name: "Gen.G", code: "GEN" },
      ],
      strategy: { type: "bestOf", count: 3 },
    },
    ...overrides,
  });
}

const completed = evt({
  startTime: "2026-04-20T08:00:00Z",
  state: "completed",
  blockName: "Week 3",
  match: {
    id: "m2",
    teams: [
      { name: "T1", code: "T1", result: { outcome: "win", gameWins: 2 } },
      { name: "Gen.G", code: "GEN", result: { outcome: "loss", gameWins: 1 } },
    ],
    strategy: { type: "bestOf", count: 3 },
  },
});

const live = evt({
  startTime: "2026-04-21T08:00:00Z",
  state: "inProgress",
  match: {
    id: "m3",
    teams: [
      { name: "Hanwha Life", code: "HLE", result: { gameWins: 1 } },
      { name: "DRX", code: "DRX", result: { gameWins: 0 } },
    ],
    strategy: { type: "bestOf", count: 3 },
  },
});

describe("formatIctTime / formatIctDayLabel", () => {
  it("formats UTC datetime in ICT", () => {
    expect(formatIctTime(new Date("2026-04-21T09:00:00Z"))).toBe("16:00");
    expect(formatIctDayLabel(new Date("2026-04-21T00:00:00Z"))).toBe("Tue Apr 21");
  });
});

describe("formatEventLine", () => {
  it("omits league name by default (renders under league header)", () => {
    expect(formatEventLine(evt())).not.toContain("LCK");
  });

  it("includes league name when showLeague is true", () => {
    expect(formatEventLine(evt(), { showLeague: true })).toContain("LCK");
  });

  it("renders completed with bolded winner + score", () => {
    const line = formatEventLine(completed);
    expect(line.startsWith("✅")).toBe(true);
    expect(line).toContain("<b>T1</b>");
    expect(line).toContain("2–1");
    expect(line).toContain("GEN");
    expect(line).toContain("Bo3");
    expect(line).toContain("Week 3");
  });

  it("renders live with LIVE prefix + current score", () => {
    const line = formatEventLine(live);
    expect(line.startsWith("🔴 LIVE")).toBe(true);
    expect(line).toContain("1–0");
  });

  it("renders unstarted with ICT time + vs", () => {
    const line = formatEventLine(evt());
    expect(line.startsWith("🕒 16:00")).toBe(true);
    expect(line).toContain("T1 vs GEN");
  });

  it("escapes HTML in block and team labels", () => {
    const line = formatEventLine(
      evt({
        blockName: "<script>",
        match: {
          id: "x",
          teams: [{ code: "<bad>" }, { code: "GEN" }],
          strategy: { type: "bestOf", count: 3 },
        },
      }),
    );
    expect(line).not.toContain("<script>");
    expect(line).toContain("&lt;script&gt;");
    expect(line).toContain("&lt;bad&gt;");
  });

  it("shows TBD when a team is missing", () => {
    const line = formatEventLine(
      evt({
        match: {
          id: "x",
          teams: [null, { code: "GEN" }],
          strategy: { type: "bestOf", count: 3 },
        },
      }),
    );
    expect(line).toContain("TBD vs GEN");
  });
});

describe("renderToday", () => {
  it("empty state when no events", () => {
    expect(renderToday([], new Date("2026-04-21T00:00:00Z"))).toContain("No matches today.");
  });

  it("groups by league with a header per league", () => {
    const events = [
      evt({ league: { name: "LPL", slug: "lpl" }, startTime: "2026-04-21T07:00:00Z" }),
      evt({ league: { name: "LCK", slug: "lck" }, startTime: "2026-04-21T08:00:00Z" }),
      evt({ league: { name: "LCK", slug: "lck" }, startTime: "2026-04-21T09:00:00Z" }),
    ];
    const out = renderToday(events, new Date("2026-04-21T00:00:00Z"));
    // LCK section appears before LPL because LEAGUE_ORDER ranks LCK higher
    expect(out).toMatch(/<b>LCK<\/b>[\s\S]*<b>LPL<\/b>/);
    // Two lines under LCK, one under LPL
    expect((out.match(/🕒/g) || []).length).toBe(3);
  });

  it("ranks known leagues before unknown slug", () => {
    const events = [
      evt({ league: { name: "Foo Cup", slug: "foo-cup" } }),
      evt({ league: { name: "LCK", slug: "lck" } }),
    ];
    const out = renderToday(events, new Date("2026-04-21T00:00:00Z"));
    expect(out.indexOf("<b>LCK</b>")).toBeLessThan(out.indexOf("<b>Foo Cup</b>"));
  });
});

describe("renderWeek", () => {
  it("empty state when no events", () => {
    expect(
      renderWeek([], new Date("2026-04-21T00:00:00Z"), new Date("2026-04-28T00:00:00Z")),
    ).toContain("No matches this week.");
  });

  it("nests leagues under each ICT day in chronological order", () => {
    const events = [
      evt({
        startTime: "2026-04-21T09:00:00Z",
        league: { name: "LCK", slug: "lck" },
      }),
      evt({
        startTime: "2026-04-22T09:00:00Z",
        league: { name: "LPL", slug: "lpl" },
      }),
      evt({
        startTime: "2026-04-22T11:00:00Z",
        league: { name: "LCK", slug: "lck" },
      }),
    ];
    const out = renderWeek(
      events,
      new Date("2026-04-21T00:00:00Z"),
      new Date("2026-04-28T00:00:00Z"),
    );
    expect(out.indexOf("Apr 21")).toBeLessThan(out.indexOf("Apr 22"));
    // Apr 22: LCK section appears before LPL (per LEAGUE_ORDER)
    const apr22Block = out.split("Apr 22")[1] || "";
    expect(apr22Block.indexOf("<b>LCK</b>")).toBeLessThan(apr22Block.indexOf("<b>LPL</b>"));
  });
});
