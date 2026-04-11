import { describe, expect, it } from "vitest";
import { validateCommand } from "../../src/modules/validate-command.js";

const noop = async () => {};
const base = (overrides = {}) => ({
  name: "foo",
  visibility: "public",
  description: "does foo",
  handler: noop,
  ...overrides,
});

describe("validateCommand", () => {
  it("accepts valid public/protected/private commands", () => {
    expect(() => validateCommand(base(), "m")).not.toThrow();
    expect(() => validateCommand(base({ visibility: "protected" }), "m")).not.toThrow();
    expect(() => validateCommand(base({ visibility: "private" }), "m")).not.toThrow();
  });

  it("rejects invalid visibility", () => {
    expect(() => validateCommand(base({ visibility: "secret" }), "m")).toThrow(/visibility/);
  });

  it("rejects leading slash in name", () => {
    expect(() => validateCommand(base({ name: "/foo" }), "m")).toThrow();
  });

  it("rejects uppercase in name", () => {
    expect(() => validateCommand(base({ name: "Foo" }), "m")).toThrow();
  });

  it("rejects name > 32 chars", () => {
    expect(() => validateCommand(base({ name: "a".repeat(33) }), "m")).toThrow();
  });

  it("rejects missing or empty description for all visibilities", () => {
    expect(() => validateCommand(base({ description: "" }), "m")).toThrow(/description/);
    expect(() => validateCommand(base({ visibility: "private", description: "" }), "m")).toThrow(
      /description/,
    );
  });

  it("rejects description > 256 chars", () => {
    expect(() => validateCommand(base({ description: "x".repeat(257) }), "m")).toThrow(/256/);
  });

  it("rejects non-function handler", () => {
    expect(() => validateCommand(base({ handler: null }), "m")).toThrow(/handler/);
  });

  it("error messages include module + command name", () => {
    try {
      validateCommand(base({ name: "Bad" }), "wordle");
    } catch (err) {
      expect(err.message).toContain("wordle");
      expect(err.message).toContain("Bad");
    }
  });
});
