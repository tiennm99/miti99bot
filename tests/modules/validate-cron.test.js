import { describe, expect, it } from "vitest";
import { validateCron } from "../../src/modules/validate-cron.js";

const noop = async () => {};
const base = (overrides = {}) => ({
  name: "nightly",
  schedule: "0 1 * * *",
  handler: noop,
  ...overrides,
});

describe("validateCron", () => {
  it("accepts a valid cron entry", () => {
    expect(() => validateCron(base(), "mod")).not.toThrow();
  });

  it("accepts 6-field schedule (with seconds)", () => {
    expect(() => validateCron(base({ schedule: "0 0 1 * * *" }), "mod")).not.toThrow();
  });

  it("accepts names with hyphens and underscores", () => {
    expect(() => validateCron(base({ name: "my-cron_job" }), "mod")).not.toThrow();
  });

  it("rejects non-object entry", () => {
    expect(() => validateCron(null, "mod")).toThrow(/not an object/);
    expect(() => validateCron("string", "mod")).toThrow(/not an object/);
  });

  it("rejects name that fails pattern", () => {
    expect(() => validateCron(base({ name: "Bad Name!" }), "mod")).toThrow(/name must match/);
    expect(() => validateCron(base({ name: "" }), "mod")).toThrow(/name must match/);
    expect(() => validateCron(base({ name: "a".repeat(33) }), "mod")).toThrow(/name must match/);
  });

  it("rejects empty schedule", () => {
    expect(() => validateCron(base({ schedule: "" }), "mod")).toThrow(/non-empty/);
    expect(() => validateCron(base({ schedule: "   " }), "mod")).toThrow(/non-empty/);
  });

  it("rejects schedule with wrong field count", () => {
    expect(() => validateCron(base({ schedule: "* * * *" }), "mod")).toThrow(/cron expression/);
    expect(() => validateCron(base({ schedule: "not-a-cron" }), "mod")).toThrow(/cron expression/);
  });

  it("rejects non-function handler", () => {
    expect(() => validateCron(base({ handler: null }), "mod")).toThrow(/handler/);
    expect(() => validateCron(base({ handler: "fn" }), "mod")).toThrow(/handler/);
  });

  it("error messages include module name and cron name", () => {
    try {
      validateCron(base({ handler: null }), "trading");
    } catch (err) {
      expect(err.message).toContain("trading");
      expect(err.message).toContain("nightly");
    }
  });
});
