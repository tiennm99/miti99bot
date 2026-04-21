import { describe, expect, it } from "vitest";
import {
  classifyMatch,
  formatMatchLine,
  parseUtc,
  renderToday,
  renderWeek,
} from "../../../src/modules/lolschedule/format.js";

const scheduled = {
  DateTime: "2026-04-21 09:00:00",
  T1: "T1",
  T2: "Gen.G",
  S1: "",
  S2: "",
  Winner: "",
  Tournament: "LCK 2026 Spring",
  BO: "3",
  OP: "LCK/2026_Season/Spring_Season",
};

const live = {
  ...scheduled,
  DateTime: "2026-04-21 08:00:00",
  S1: "1",
  S2: "0",
};

const played = {
  ...scheduled,
  DateTime: "2026-04-20 10:00:00",
  S1: "2",
  S2: "1",
  Winner: "1",
};

// Fixed reference — 2026-04-21 08:30:00 UTC (i.e. 15:30 ICT)
const nowMs = Date.parse("2026-04-21T08:30:00Z");

describe("parseUtc", () => {
  it("parses Leaguepedia UTC literal", () => {
    expect(parseUtc("2026-04-21 09:00:00").toISOString()).toBe("2026-04-21T09:00:00.000Z");
  });
});

describe("classifyMatch", () => {
  it("returns played when Winner is 1 or 2", () => {
    expect(classifyMatch(played, nowMs)).toBe("played");
    expect(classifyMatch({ ...played, Winner: "2" }, nowMs)).toBe("played");
  });

  it("returns live when started, has partial score, no winner", () => {
    expect(classifyMatch(live, nowMs)).toBe("live");
  });

  it("returns scheduled when start time is in the future", () => {
    expect(classifyMatch(scheduled, nowMs)).toBe("scheduled");
  });

  it("returns scheduled when start is past but no score", () => {
    const row = { ...scheduled, DateTime: "2026-04-21 07:00:00" };
    expect(classifyMatch(row, nowMs)).toBe("scheduled");
  });
});

describe("formatMatchLine", () => {
  it("renders scheduled with ICT time and vs", () => {
    const line = formatMatchLine(scheduled, nowMs);
    // 09:00 UTC → 16:00 ICT
    expect(line).toContain("16:00");
    expect(line).toContain("vs");
    expect(line).toContain("Bo3");
    expect(line).toContain("LCK 2026 Spring");
    expect(line.startsWith("🕒")).toBe(true);
  });

  it("renders played with score and winner bolded", () => {
    const line = formatMatchLine(played, nowMs);
    expect(line).toContain("2–1");
    expect(line).toContain("<b>T1</b>");
    expect(line.startsWith("✅")).toBe(true);
  });

  it("renders live prefix", () => {
    const line = formatMatchLine(live, nowMs);
    expect(line.startsWith("🔴 LIVE")).toBe(true);
    expect(line).toContain("1–0");
  });

  it("escapes HTML in team and tournament names", () => {
    const row = { ...scheduled, T1: "<script>", Tournament: "A&B" };
    const line = formatMatchLine(row, nowMs);
    expect(line).not.toContain("<script>");
    expect(line).toContain("&lt;script&gt;");
    expect(line).toContain("A&amp;B");
  });

  it("shows TBD when team field is empty", () => {
    const row = { ...scheduled, T1: "" };
    expect(formatMatchLine(row, nowMs)).toContain("TBD");
  });
});

describe("renderToday", () => {
  it("returns empty-state message when no rows", () => {
    const out = renderToday([], new Date("2026-04-21T00:00:00Z"), nowMs);
    expect(out).toContain("No matches today.");
  });

  it("renders header + one line per match", () => {
    const out = renderToday([scheduled, played], new Date("2026-04-21T00:00:00Z"), nowMs);
    expect(out).toContain("<b>LoL —");
    expect(out.split("\n").length).toBeGreaterThanOrEqual(3);
  });
});

describe("renderWeek", () => {
  it("groups rows by ICT day", () => {
    const rows = [
      { ...scheduled, DateTime: "2026-04-21 09:00:00" }, // Apr 21 ICT
      { ...scheduled, DateTime: "2026-04-22 10:00:00" }, // Apr 22 ICT
      { ...scheduled, DateTime: "2026-04-22 12:00:00" }, // Apr 22 ICT
    ];
    const out = renderWeek(
      rows,
      new Date("2026-04-21T00:00:00Z"),
      new Date("2026-04-28T00:00:00Z"),
      nowMs,
    );
    // Day labels appear once each, in order
    expect(out.indexOf("Apr 21")).toBeLessThan(out.indexOf("Apr 22"));
    const apr22Matches = out.split("Apr 22")[1] || "";
    expect((apr22Matches.match(/🕒/g) || []).length).toBe(2);
  });

  it("returns empty-state when no matches", () => {
    const out = renderWeek(
      [],
      new Date("2026-04-21T00:00:00Z"),
      new Date("2026-04-28T00:00:00Z"),
      nowMs,
    );
    expect(out).toContain("No matches this week.");
  });
});
