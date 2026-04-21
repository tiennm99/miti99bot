import { describe, expect, it } from "vitest";
import {
  formatEventLine,
  formatIctDayLabel,
  formatIctTime,
  renderToday,
  renderWeek,
} from "../../../src/modules/lolschedule/format.js";

/** @typedef {import("../../../src/modules/lolschedule/api-client.js").ScheduleEvent} ScheduleEvent */

const completed = /** @type {ScheduleEvent} */ ({
  startTime: "2026-04-20T08:00:00Z",
  state: "completed",
  blockName: "Week 3",
  league: { name: "LCK", slug: "lck" },
  match: {
    id: "1",
    teams: [
      { name: "T1", code: "T1", result: { outcome: "win", gameWins: 2 } },
      { name: "Gen.G", code: "GEN", result: { outcome: "loss", gameWins: 1 } },
    ],
    strategy: { type: "bestOf", count: 3 },
  },
});

const live = /** @type {ScheduleEvent} */ ({
  startTime: "2026-04-21T08:00:00Z",
  state: "inProgress",
  blockName: "Week 4",
  league: { name: "LCK", slug: "lck" },
  match: {
    id: "2",
    teams: [
      { name: "Hanwha Life", code: "HLE", result: { gameWins: 1 } },
      { name: "DRX", code: "DRX", result: { gameWins: 0 } },
    ],
    strategy: { type: "bestOf", count: 3 },
  },
});

const scheduled = /** @type {ScheduleEvent} */ ({
  startTime: "2026-04-21T09:00:00Z", // 16:00 ICT
  state: "unstarted",
  blockName: "Week 4",
  league: { name: "LCK", slug: "lck" },
  match: {
    id: "3",
    teams: [
      { name: "KT Rolster", code: "KT" },
      { name: "Dplus KIA", code: "DK" },
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
  it("renders completed with bolded winner + score", () => {
    const line = formatEventLine(completed);
    expect(line.startsWith("✅")).toBe(true);
    expect(line).toContain("<b>T1</b>");
    expect(line).toContain("2–1");
    expect(line).toContain("GEN");
    expect(line).toContain("Bo3");
    expect(line).toContain("LCK");
    expect(line).toContain("Week 3");
  });

  it("renders live with LIVE prefix + current score", () => {
    const line = formatEventLine(live);
    expect(line.startsWith("🔴 LIVE")).toBe(true);
    expect(line).toContain("1–0");
  });

  it("renders unstarted with ICT time + vs", () => {
    const line = formatEventLine(scheduled);
    expect(line.startsWith("🕒 16:00")).toBe(true);
    expect(line).toContain("KT vs DK");
  });

  it("escapes HTML in league, block, team names", () => {
    const event = /** @type {ScheduleEvent} */ ({
      ...scheduled,
      league: { name: "A&B", slug: "ab" },
      blockName: "<script>",
      match: {
        ...scheduled.match,
        teams: [{ code: "<bad>" }, { code: "GEN" }],
      },
    });
    const line = formatEventLine(event);
    expect(line).not.toContain("<script>");
    expect(line).toContain("&lt;script&gt;");
    expect(line).toContain("&lt;bad&gt;");
    expect(line).toContain("A&amp;B");
  });

  it("shows TBD when team is missing", () => {
    const event = /** @type {ScheduleEvent} */ ({
      ...scheduled,
      match: { ...scheduled.match, teams: [null, { code: "GEN" }] },
    });
    expect(formatEventLine(event)).toContain("TBD vs GEN");
  });
});

describe("renderToday", () => {
  it("empty state when no events", () => {
    expect(renderToday([], new Date("2026-04-21T00:00:00Z"))).toContain("No matches today.");
  });

  it("renders header + one line per event", () => {
    const out = renderToday([scheduled, live], new Date("2026-04-21T00:00:00Z"));
    expect(out).toContain("<b>LoL — Tue Apr 21</b>");
    expect(out.split("\n")).toHaveLength(3);
  });
});

describe("renderWeek", () => {
  it("groups events by ICT day in order", () => {
    const events = [
      { ...scheduled, startTime: "2026-04-21T09:00:00Z" },
      { ...scheduled, startTime: "2026-04-22T10:00:00Z" },
      { ...scheduled, startTime: "2026-04-22T12:00:00Z" },
    ];
    const out = renderWeek(
      events,
      new Date("2026-04-21T00:00:00Z"),
      new Date("2026-04-28T00:00:00Z"),
    );
    expect(out.indexOf("Apr 21")).toBeLessThan(out.indexOf("Apr 22"));
    const apr22Section = out.split("Apr 22")[1] || "";
    expect((apr22Section.match(/🕒/g) || []).length).toBe(2);
  });

  it("empty state when no events", () => {
    expect(
      renderWeek([], new Date("2026-04-21T00:00:00Z"), new Date("2026-04-28T00:00:00Z")),
    ).toContain("No matches this week.");
  });
});
