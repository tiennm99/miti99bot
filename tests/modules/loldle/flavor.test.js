import { describe, expect, it } from "vitest";
import { attemptFlavor, formatDuration } from "../../../src/modules/loldle/flavor.js";

describe("attemptFlavor", () => {
  it("rewards a first-try solve", () => {
    expect(attemptFlavor(1, 8)).toBe("First try!");
  });

  it("calls out a second-attempt solve as Sharp", () => {
    expect(attemptFlavor(2, 8)).toBe("Sharp!");
  });

  it("flags the last guess as Phew", () => {
    expect(attemptFlavor(8, 8)).toBe("Phew — last one!");
  });

  it("calls the second-to-last and third-to-last a close call", () => {
    expect(attemptFlavor(6, 8)).toBe("Close call!");
    expect(attemptFlavor(7, 8)).toBe("Close call!");
  });

  it("falls back to Nice for the middle range", () => {
    expect(attemptFlavor(3, 8)).toBe("Nice.");
    expect(attemptFlavor(5, 8)).toBe("Nice.");
  });
});

describe("formatDuration", () => {
  it("renders sub-minute as seconds only", () => {
    expect(formatDuration(42_000)).toBe("42s");
    expect(formatDuration(999)).toBe("1s");
    expect(formatDuration(0)).toBe("0s");
  });

  it("renders minutes with seconds, dropping seconds when zero", () => {
    expect(formatDuration(3 * 60_000 + 14_000)).toBe("3m 14s");
    expect(formatDuration(5 * 60_000)).toBe("5m");
  });

  it("renders hours with minutes, dropping minutes when zero", () => {
    expect(formatDuration(60 * 60_000 + 12 * 60_000)).toBe("1h 12m");
    expect(formatDuration(2 * 60 * 60_000)).toBe("2h");
  });

  it("clamps negative input to 0s (clock-skew safety)", () => {
    expect(formatDuration(-5_000)).toBe("0s");
  });
});
