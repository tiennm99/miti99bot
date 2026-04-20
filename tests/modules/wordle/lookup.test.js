import { describe, expect, it } from "vitest";
import { makeWordSet, normalizeWord, validateGuess } from "../../../src/modules/wordle/lookup.js";

const dict = makeWordSet(["crane", "shale", "abbey", "lever"]);

describe("normalizeWord", () => {
  it("lowercases and strips non-letters", () => {
    expect(normalizeWord("  CRANE  ")).toBe("crane");
    expect(normalizeWord("cr-ane")).toBe("crane");
    expect(normalizeWord("cr4ne")).toBe("crne");
  });

  it("handles null/undefined/empty", () => {
    expect(normalizeWord("")).toBe("");
    expect(normalizeWord(null)).toBe("");
    expect(normalizeWord(undefined)).toBe("");
  });
});

describe("validateGuess", () => {
  it("accepts in-dictionary 5-letter words", () => {
    expect(validateGuess(dict, "crane")).toEqual({ ok: true, word: "crane" });
    expect(validateGuess(dict, "CRANE")).toEqual({ ok: true, word: "crane" });
  });

  it("rejects empty input as 'empty'", () => {
    expect(validateGuess(dict, "")).toMatchObject({ ok: false, reason: "empty" });
    expect(validateGuess(dict, "---")).toMatchObject({ ok: false, reason: "empty" });
  });

  it("rejects wrong length as 'length'", () => {
    expect(validateGuess(dict, "cat")).toMatchObject({ ok: false, reason: "length" });
    expect(validateGuess(dict, "catnaps")).toMatchObject({ ok: false, reason: "length" });
  });

  it("rejects unknown word as 'unknown'", () => {
    expect(validateGuess(dict, "zzzzz")).toMatchObject({ ok: false, reason: "unknown" });
  });
});
