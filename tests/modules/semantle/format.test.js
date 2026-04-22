import { describe, expect, it } from "vitest";
import { formatWarmth, warmthEmoji } from "../../../src/modules/semantle/format.js";

describe("semantle/format", () => {
  describe("formatWarmth", () => {
    it("formats positive similarity as signed percent with padding", () => {
      expect(formatWarmth(0.734)).toBe("+73");
      expect(formatWarmth(1.0)).toBe("+100");
      expect(formatWarmth(0.05)).toBe("+05");
    });

    it("formats negative similarity with minus sign and padding", () => {
      expect(formatWarmth(-0.04)).toBe("-04");
      expect(formatWarmth(-1.0)).toBe("-100");
      expect(formatWarmth(-0.5)).toBe("-50");
    });

    it("formats zero as +00", () => {
      expect(formatWarmth(0)).toBe("+00");
      expect(formatWarmth(0.0)).toBe("+00");
    });

    it("rounds to nearest integer", () => {
      expect(formatWarmth(0.504)).toBe("+50");
      expect(formatWarmth(0.505)).toBe("+51");
      expect(formatWarmth(-0.125)).toBe("-12");
    });

    it("handles boundary values", () => {
      expect(formatWarmth(0.004)).toBe("+00");
      expect(formatWarmth(0.994)).toBe("+99");
    });
  });

  describe("warmthEmoji", () => {
    it("returns 🥶 for similarity < 0.2", () => {
      expect(warmthEmoji(0.19)).toBe("🥶");
      expect(warmthEmoji(-1)).toBe("🥶");
      expect(warmthEmoji(0)).toBe("🥶");
    });

    it("returns 😐 for similarity >= 0.2 and < 0.4", () => {
      expect(warmthEmoji(0.2)).toBe("😐");
      expect(warmthEmoji(0.3)).toBe("😐");
      expect(warmthEmoji(0.39)).toBe("😐");
    });

    it("returns 🌡️ for similarity >= 0.4 and < 0.6", () => {
      expect(warmthEmoji(0.4)).toBe("🌡️");
      expect(warmthEmoji(0.5)).toBe("🌡️");
      expect(warmthEmoji(0.59)).toBe("🌡️");
    });

    it("returns 🔥 for similarity >= 0.6 and < 0.8", () => {
      expect(warmthEmoji(0.6)).toBe("🔥");
      expect(warmthEmoji(0.7)).toBe("🔥");
      expect(warmthEmoji(0.79)).toBe("🔥");
    });

    it("returns 🎯 for similarity >= 0.8", () => {
      expect(warmthEmoji(0.8)).toBe("🎯");
      expect(warmthEmoji(0.9)).toBe("🎯");
      expect(warmthEmoji(1)).toBe("🎯");
    });

    it("handles edge cases at boundaries", () => {
      expect(warmthEmoji(0.1999)).toBe("🥶");
      expect(warmthEmoji(0.2001)).toBe("😐");
      expect(warmthEmoji(0.7999)).toBe("🔥");
      expect(warmthEmoji(0.8001)).toBe("🎯");
    });
  });
});
