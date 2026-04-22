import { describe, expect, it } from "vitest";
import { calibrate, formatWarmth, warmthEmoji } from "../../../src/modules/semantle/format.js";

describe("semantle/format", () => {
  describe("calibrate", () => {
    it("maps raw cosine <= floor to 0", () => {
      expect(calibrate(0.4)).toBe(0);
      expect(calibrate(0.2)).toBe(0);
      expect(calibrate(-1)).toBe(0);
    });

    it("maps raw cosine = 1 to 100", () => {
      expect(calibrate(1)).toBe(100);
    });

    it("is monotonically increasing between floor and 1", () => {
      let prev = calibrate(0.4);
      for (let r = 0.41; r <= 1.001; r += 0.02) {
        const s = calibrate(r);
        expect(s).toBeGreaterThanOrEqual(prev);
        prev = s;
      }
    });

    it("compresses mid-range cosines so unrelated-baseline reads low", () => {
      // Unrelated BGE pairs cluster around 0.45-0.55 — should still look cold.
      expect(calibrate(0.5)).toBeLessThan(25);
      expect(calibrate(0.55)).toBeLessThan(35);
    });

    it("rewards clearly-related cosines with high scores", () => {
      expect(calibrate(0.75)).toBeGreaterThan(70);
      expect(calibrate(0.85)).toBeGreaterThan(85);
      expect(calibrate(0.95)).toBeGreaterThan(95);
    });

    it("stays clamped to [0, 100]", () => {
      expect(calibrate(2)).toBe(100);
      expect(calibrate(-5)).toBe(0);
    });
  });

  describe("formatWarmth", () => {
    it("formats integer percent with zero-padding at width 2", () => {
      expect(formatWarmth(0)).toBe("00");
      expect(formatWarmth(7)).toBe("07");
      expect(formatWarmth(73)).toBe("73");
    });

    it("returns '100' without padding at the max", () => {
      expect(formatWarmth(100)).toBe("100");
    });

    it("rounds to nearest integer", () => {
      expect(formatWarmth(50.4)).toBe("50");
      expect(formatWarmth(50.5)).toBe("51");
      expect(formatWarmth(99.5)).toBe("100");
    });
  });

  describe("warmthEmoji", () => {
    it("returns 🥶 for score < 15", () => {
      expect(warmthEmoji(0)).toBe("🥶");
      expect(warmthEmoji(14.9)).toBe("🥶");
    });

    it("returns 😐 for score in [15, 40)", () => {
      expect(warmthEmoji(15)).toBe("😐");
      expect(warmthEmoji(30)).toBe("😐");
      expect(warmthEmoji(39.9)).toBe("😐");
    });

    it("returns 🌡️ for score in [40, 70)", () => {
      expect(warmthEmoji(40)).toBe("🌡️");
      expect(warmthEmoji(55)).toBe("🌡️");
      expect(warmthEmoji(69.9)).toBe("🌡️");
    });

    it("returns 🔥 for score in [70, 90)", () => {
      expect(warmthEmoji(70)).toBe("🔥");
      expect(warmthEmoji(80)).toBe("🔥");
      expect(warmthEmoji(89.9)).toBe("🔥");
    });

    it("returns 🎯 for score >= 90", () => {
      expect(warmthEmoji(90)).toBe("🎯");
      expect(warmthEmoji(99)).toBe("🎯");
      expect(warmthEmoji(100)).toBe("🎯");
    });
  });
});
