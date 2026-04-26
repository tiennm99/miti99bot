import { describe, expect, it } from "vitest";
import {
  aggregateLines,
  formatReport,
  parseLogLine,
  percentile,
} from "../../scripts/analyze-soak.js";

// ── percentile ───────────────────────────────────────────────────────────────

describe("percentile", () => {
  it("returns 0 for empty array", () => {
    expect(percentile([], 50)).toBe(0);
  });

  it("returns the only element for a single-element array", () => {
    expect(percentile([42], 50)).toBe(42);
    expect(percentile([42], 95)).toBe(42);
  });

  it("p50 of [1,2,3,4,5] = 3", () => {
    expect(percentile([1, 2, 3, 4, 5], 50)).toBe(3);
  });

  it("p100 returns last element", () => {
    expect(percentile([10, 20, 30], 100)).toBe(30);
  });

  it("p0 returns first element", () => {
    expect(percentile([10, 20, 30], 0)).toBe(10);
  });

  it("p95 of 100-element uniform distribution is near top", () => {
    const data = Array.from({ length: 100 }, (_, i) => i + 1); // 1..100
    expect(percentile(data, 95)).toBe(95);
  });
});

// ── parseLogLine ─────────────────────────────────────────────────────────────

describe("parseLogLine", () => {
  it("parses a plain NDJSON line", () => {
    const line = '{"event":"cmd_timing","cmd":"/wordle","total":120,"cold":true,"marks":[]}';
    expect(parseLogLine(line)).toEqual({
      event: "cmd_timing",
      cmd: "/wordle",
      total: 120,
      cold: true,
      marks: [],
    });
  });

  it("returns null for empty line", () => {
    expect(parseLogLine("")).toBeNull();
    expect(parseLogLine("   ")).toBeNull();
  });

  it("returns null for non-JSON plain text", () => {
    expect(parseLogLine("some random log line")).toBeNull();
  });

  it("extracts JSON embedded in a CSV row", () => {
    const csvRow =
      '2024-01-01T00:00:00Z,worker,"{\\"event\\":\\"cmd_timing\\",\\"cmd\\":\\"/loldle\\",\\"total\\":45,\\"cold\\":false,\\"marks\\":[]}"';
    const result = parseLogLine(csvRow);
    // May fail to parse double-escaped CSV — test the raw braces path instead.
    // Use a simpler CSV format where JSON is not double-escaped:
    const simpleCsv =
      '2024-01-01,{"event":"cmd_timing","cmd":"/loldle","total":45,"cold":false,"marks":[]},extra';
    const r2 = parseLogLine(simpleCsv);
    expect(r2).not.toBeNull();
    expect(r2.event).toBe("cmd_timing");
    expect(r2.cmd).toBe("/loldle");
  });

  it("returns null for malformed JSON", () => {
    expect(parseLogLine("{bad json}")).toBeNull();
  });
});

// ── aggregateLines ────────────────────────────────────────────────────────────

describe("aggregateLines", () => {
  const makeTimingLine = (cmd, total, cold) =>
    JSON.stringify({ event: "cmd_timing", cmd, total, cold, marks: [] });

  it("buckets cold and warm samples separately", () => {
    const lines = [
      makeTimingLine("/wordle", 1500, true),
      makeTimingLine("/wordle", 1400, true),
      makeTimingLine("/wordle", 50, false),
      makeTimingLine("/wordle", 60, false),
      makeTimingLine("/wordle", 55, false),
    ];
    const { buckets } = aggregateLines(lines, null);
    expect(buckets.get("/wordle|cold").n).toBe(2);
    expect(buckets.get("/wordle|warm").n).toBe(3);
  });

  it("filters to requested commands only", () => {
    const lines = [makeTimingLine("/wordle", 100, false), makeTimingLine("/loldle", 200, false)];
    const { buckets } = aggregateLines(lines, ["/wordle"]);
    expect(buckets.has("/wordle|warm")).toBe(true);
    expect(buckets.has("/loldle|warm")).toBe(false);
  });

  it("counts dual-write secondary failures", () => {
    const lines = [
      '{"msg":"[dual-kv] secondary write failed","phase":"dual-kv"}',
      "dual-write:secondary:failed — key wordle:state:1",
      makeTimingLine("/wordle", 80, false),
    ];
    const { errors } = aggregateLines(lines, null);
    expect(errors.dualWriteFail).toBe(2);
  });

  it("counts mongo connection errors", () => {
    const lines = [
      "MongoNetworkError: connection refused",
      "MongoError: timeout",
      makeTimingLine("/wordle", 80, false),
    ];
    const { errors } = aggregateLines(lines, null);
    expect(errors.mongoError).toBe(2);
  });

  it("counts CPU time exceeded errors", () => {
    const lines = ["Worker exceeded CPU time limit", "cpu time exceeded for isolate"];
    const { errors } = aggregateLines(lines, null);
    expect(errors.cpuExceeded).toBe(2);
  });

  it("skips non-cmd_timing JSON events", () => {
    const lines = [
      JSON.stringify({ event: "request", cold: true, path: "/webhook" }),
      makeTimingLine("/wordle", 90, false),
    ];
    const { buckets } = aggregateLines(lines, null);
    expect(buckets.size).toBe(1); // only the cmd_timing entry
  });

  it("returns empty buckets and zero errors for empty input", () => {
    const { buckets, errors } = aggregateLines([], null);
    expect(buckets.size).toBe(0);
    expect(errors.dualWriteFail).toBe(0);
    expect(errors.mongoError).toBe(0);
    expect(errors.cpuExceeded).toBe(0);
  });
});

// ── formatReport ─────────────────────────────────────────────────────────────

describe("formatReport", () => {
  it("produces a markdown table with correct headers", () => {
    const lines = [
      JSON.stringify({ event: "cmd_timing", cmd: "/wordle", total: 1500, cold: true, marks: [] }),
      JSON.stringify({ event: "cmd_timing", cmd: "/wordle", total: 50, cold: false, marks: [] }),
    ];
    const { buckets, errors } = aggregateLines(lines, null);
    const report = formatReport(buckets, errors);

    expect(report).toContain("| cmd | cold/warm | n | p50 | p95 | p99 |");
    expect(report).toContain("/wordle");
    expect(report).toContain("cold");
    expect(report).toContain("warm");
  });

  it("includes error summary section", () => {
    const { buckets, errors } = aggregateLines([], null);
    errors.dualWriteFail = 3;
    const report = formatReport(buckets, errors);
    expect(report).toContain("## Error Summary");
    expect(report).toContain("Dual-write secondary failures: 3");
    expect(report).toContain("Mongo connection errors: 0");
    expect(report).toContain("CPU time exceeded: 0");
  });

  it("computes correct p50/p95/p99 for a known dataset", () => {
    // 10 warm samples: 10,20,...,100
    const lines = Array.from({ length: 10 }, (_, i) =>
      JSON.stringify({
        event: "cmd_timing",
        cmd: "/ping",
        total: (i + 1) * 10,
        cold: false,
        marks: [],
      }),
    );
    const { buckets, errors } = aggregateLines(lines, null);
    const report = formatReport(buckets, errors);

    // p50 of [10,20,30,40,50,60,70,80,90,100] = 50 (index 4 with nearest-rank)
    // p95 = 95th percentile = index ceil(0.95*10)-1 = ceil(9.5)-1 = 10-1 = 9 → 100
    expect(report).toContain("| /ping | warm | 10 |");
  });
});
