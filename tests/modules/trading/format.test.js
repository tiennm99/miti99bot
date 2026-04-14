import { describe, expect, it } from "vitest";
import {
  formatCrypto,
  formatCurrency,
  formatPnL,
  formatStock,
  formatUSD,
  formatVND,
} from "../../../src/modules/trading/format.js";

describe("trading/format", () => {
  describe("formatVND", () => {
    it("formats with dot thousands separator", () => {
      expect(formatVND(15000000)).toBe("15.000.000 VND");
    });
    it("handles zero", () => {
      expect(formatVND(0)).toBe("0 VND");
    });
    it("handles negative", () => {
      expect(formatVND(-1500000)).toBe("-1.500.000 VND");
    });
    it("rounds to integer", () => {
      expect(formatVND(1234.56)).toBe("1.235 VND");
    });
    it("handles small numbers", () => {
      expect(formatVND(500)).toBe("500 VND");
    });
  });

  describe("formatUSD", () => {
    it("formats with comma separator and 2 decimals", () => {
      expect(formatUSD(1234.5)).toBe("$1,234.50");
    });
    it("handles zero", () => {
      expect(formatUSD(0)).toBe("$0.00");
    });
  });

  describe("formatCrypto", () => {
    it("strips trailing zeros", () => {
      expect(formatCrypto(0.001)).toBe("0.001");
      expect(formatCrypto(1.0)).toBe("1");
    });
    it("preserves up to 8 decimals", () => {
      expect(formatCrypto(0.12345678)).toBe("0.12345678");
    });
  });

  describe("formatStock", () => {
    it("floors to integer", () => {
      expect(formatStock(1.7)).toBe("1");
      expect(formatStock(150)).toBe("150");
    });
  });

  describe("formatCurrency", () => {
    it("dispatches to VND formatter", () => {
      expect(formatCurrency(5000, "VND")).toBe("5.000 VND");
    });
    it("dispatches to USD formatter", () => {
      expect(formatCurrency(100.5, "USD")).toBe("$100.50");
    });
    it("falls back for unknown currency", () => {
      expect(formatCurrency(42, "EUR")).toBe("42 EUR");
    });
  });

  describe("formatPnL", () => {
    it("shows positive P&L", () => {
      expect(formatPnL(12000000, 10000000)).toBe("+2.000.000 VND (+20.00%)");
    });
    it("shows negative P&L", () => {
      expect(formatPnL(8000000, 10000000)).toBe("-2.000.000 VND (-20.00%)");
    });
    it("handles zero investment", () => {
      expect(formatPnL(0, 0)).toBe("+0 VND (+0.00%)");
    });
  });
});
