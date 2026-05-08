import { describe, expect, it } from "vitest";
import { parseScheduleDate } from "../../../src/modules/lolschedule/parse-date.js";

const ICT_OFFSET_MS = 7 * 60 * 60 * 1000;

/** Build a Date that represents the start of the given ICT day. */
function ictStart(year, month, day) {
  return new Date(Date.UTC(year, month - 1, day, 0, 0, 0) - ICT_OFFSET_MS);
}

/** Pick a `now` clock anchored at noon ICT on a known day. */
function nowAt(year, month, day) {
  return new Date(Date.UTC(year, month - 1, day, 12, 0, 0) - ICT_OFFSET_MS);
}

describe("parseScheduleDate", () => {
  const now = nowAt(2026, 5, 8); // 8 May 2026 ICT

  it("returns today's ICT day start when input is empty", () => {
    const r = parseScheduleDate("", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 5, 8).getTime());
  });

  it("returns today when input is whitespace", () => {
    const r = parseScheduleDate("   ", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 5, 8).getTime());
  });

  it("returns today when input is undefined", () => {
    const r = parseScheduleDate(undefined, now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 5, 8).getTime());
  });

  it("parses dd-mm-yyyy", () => {
    const r = parseScheduleDate("01-06-2025", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2025, 6, 1).getTime());
  });

  it("parses dd/mm/yyyy", () => {
    const r = parseScheduleDate("01/06/2025", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2025, 6, 1).getTime());
  });

  it("parses ddmmyyyy (no separator)", () => {
    const r = parseScheduleDate("01062025", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2025, 6, 1).getTime());
  });

  it("fills in current year when only dd-mm given", () => {
    const r = parseScheduleDate("12-09", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 9, 12).getTime());
  });

  it("fills in current year when only dd/mm given", () => {
    const r = parseScheduleDate("12/09", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 9, 12).getTime());
  });

  it("fills in current year when only ddmm (4 digits) given", () => {
    const r = parseScheduleDate("1209", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 9, 12).getTime());
  });

  it("fills in current month and year when only dd given (2 digits)", () => {
    const r = parseScheduleDate("20", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 5, 20).getTime());
  });

  it("fills in current month and year when only d given (1 digit)", () => {
    const r = parseScheduleDate("3", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2026, 5, 3).getTime());
  });

  it("rejects a 6-digit blob (mmyyyy with no day)", () => {
    const r = parseScheduleDate("052026", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/Invalid date/);
  });

  it("rejects a 3-digit blob (ambiguous)", () => {
    const r = parseScheduleDate("123", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/Invalid date/);
  });

  it("rejects empty leading part with separator (no day)", () => {
    const r = parseScheduleDate("/05/2026", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/Invalid date/);
  });

  it("rejects empty middle part", () => {
    const r = parseScheduleDate("12//2026", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/Invalid date/);
  });

  it("rejects non-numeric input", () => {
    const r = parseScheduleDate("abc", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/Invalid date/);
  });

  it("rejects out-of-range day", () => {
    const r = parseScheduleDate("32-05-2026", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/day/);
  });

  it("rejects out-of-range month", () => {
    const r = parseScheduleDate("01-13-2026", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/month/);
  });

  it("rejects impossible calendar dates (Feb 30)", () => {
    const r = parseScheduleDate("30-02-2025", now);
    expect(r.ok).toBe(false);
    expect(r.error).toMatch(/does not exist/);
  });

  it("accepts Feb 29 in a leap year", () => {
    const r = parseScheduleDate("29-02-2024", now);
    expect(r.ok).toBe(true);
    expect(r.date.getTime()).toBe(ictStart(2024, 2, 29).getTime());
  });
});
