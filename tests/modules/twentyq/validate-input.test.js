import { describe, expect, it } from "vitest";
import { validateQuestion } from "../../../src/modules/twentyq/validate-input.js";

describe("twentyq/validate-input", () => {
  it.each([
    "is it big?",
    "are they common?",
    "does it have wheels?",
    "do you eat it?",
    "can it fly?",
    "has it ever lived?",
    "have you ridden one?",
    "was it invented in Asia?",
    "were they wooden?",
    "will it float?",
    "should I include it?",
    "could you carry it?",
    "would it fit in a bag?",
  ])("accepts yes/no opener: %s", (q) => {
    const r = validateQuestion(q);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.normalized).toBe(q);
  });

  it.each([
    "what is it?",
    "How does it work?",
    "why is the sky blue?",
    "which one is it?",
    "who made it?",
    "where does it live?",
    "when was it invented?",
    "tell me a hint",
    "describe it",
    "explain why",
  ])("rejects open-ended: %s", (q) => {
    const r = validateQuestion(q);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.reason).toMatch(/yes\/no/i);
  });

  it("rejects empty", () => {
    expect(validateQuestion("").ok).toBe(false);
  });

  it("rejects too short", () => {
    expect(validateQuestion("a").ok).toBe(false);
  });

  it("rejects too long (>200)", () => {
    expect(validateQuestion("is it ".repeat(80)).ok).toBe(false);
  });

  it("rejects non-string input", () => {
    expect(validateQuestion(undefined).ok).toBe(false);
    expect(validateQuestion(null).ok).toBe(false);
    expect(validateQuestion(42).ok).toBe(false);
  });

  it("collapses internal whitespace", () => {
    const r = validateQuestion("  is   it    big? ");
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.normalized).toBe("is it big?");
  });
});
