import { describe, expect, it } from "vitest";
import {
  MODEL_ID,
  UpstreamError,
  extractText,
  generateRoundStart,
  judge,
  normalizeJudgement,
  parseJudgementJson,
  redactSecret,
} from "../../../src/modules/twentyq/ai-client.js";
import { makeFakeAi, mockFailure, mockJudgement, mockRoundStart } from "../../fakes/fake-ai.js";

const baseState = () => ({
  category: "instrument",
  target: "organ",
  initialHint: "uses wind through pipes",
  startedAt: 1,
  solved: false,
  turns: [],
});

describe("twentyq/ai-client", () => {
  describe("extractText", () => {
    it("reads traditional Workers-AI { response } shape", () => {
      expect(extractText({ response: "hello" })).toBe("hello");
    });

    it("reads OpenAI-compatible choices[0].message.content", () => {
      expect(extractText({ choices: [{ message: { content: "world" } }] })).toBe("world");
    });

    it("concatenates array content parts", () => {
      expect(
        extractText({
          choices: [{ message: { content: [{ text: "a" }, { text: "b" }] } }],
        }),
      ).toBe("ab");
    });

    it("passes through strings", () => {
      expect(extractText("direct")).toBe("direct");
    });

    it("empty string on unknown shape", () => {
      expect(extractText(null)).toBe("");
      expect(extractText({})).toBe("");
    });
  });

  describe("parseJudgementJson", () => {
    it("parses clean one-line JSON", () => {
      const r = parseJudgementJson('{"is_guess":false,"answer":"yes","hint":"big"}');
      expect(r).toEqual({ is_guess: false, answer: "yes", hint: "big" });
    });

    it("pulls JSON out of surrounding prose", () => {
      const r = parseJudgementJson(
        'Sure, here is my answer: {"is_guess":true,"answer":"no","hint":"x"} — hope that helps!',
      );
      expect(r?.is_guess).toBe(true);
    });

    it("strips code fences", () => {
      const r = parseJudgementJson('```json\n{"is_guess":false,"answer":"yes","hint":"h"}\n```');
      expect(r?.hint).toBe("h");
    });

    it("handles nested braces inside strings", () => {
      const r = parseJudgementJson('{"is_guess":false,"answer":"no","hint":"has {braces}"}');
      expect(r?.hint).toBe("has {braces}");
    });

    it("returns null when no JSON object present", () => {
      expect(parseJudgementJson("no json here")).toBeNull();
      expect(parseJudgementJson("")).toBeNull();
      expect(parseJudgementJson(null)).toBeNull();
    });

    it("returns null on malformed JSON", () => {
      expect(parseJudgementJson("{not: valid}")).toBeNull();
    });
  });

  describe("normalizeJudgement", () => {
    it("coerces missing fields to defaults", () => {
      const j = normalizeJudgement(null);
      expect(j.is_guess).toBe(false);
      expect(j.answer).toBe("no");
      expect(j.hint).toBeTruthy();
    });

    it("forces answer into yes/no", () => {
      expect(normalizeJudgement({ answer: "YES" }).answer).toBe("yes");
      expect(normalizeJudgement({ answer: "maybe" }).answer).toBe("no");
    });

    it("only true is_guess passes through truthy", () => {
      expect(normalizeJudgement({ is_guess: 1 }).is_guess).toBe(false);
      expect(normalizeJudgement({ is_guess: true }).is_guess).toBe(true);
    });

    it("falls back to default hint when missing or empty", () => {
      expect(normalizeJudgement({ hint: "" }).hint).toMatch(/parse|yes\/no/i);
      expect(normalizeJudgement({ hint: "  " }).hint).toMatch(/parse|yes\/no/i);
    });
  });

  describe("redactSecret", () => {
    it("strips case-insensitive whole-word target", () => {
      expect(redactSecret("the organ is loud", "organ")).toContain("(redacted)");
      expect(redactSecret("ORGAN!", "organ")).toContain("(redacted)");
    });

    it("does not redact substring matches mid-word", () => {
      expect(redactSecret("organic shapes", "organ")).toBe("organic shapes");
    });

    it("safe message when entire hint is the secret", () => {
      const r = redactSecret("organ", "organ");
      expect(r).toMatch(/redacted/i);
    });
  });

  describe("judge (integration with fake AI)", () => {
    it("returns normalized judgement on happy path", async () => {
      const ai = makeFakeAi();
      mockJudgement(ai, { is_guess: false, answer: "yes", hint: "long and tall" });
      const r = await judge({ AI: ai }, baseState(), "is it big?");
      expect(ai.run).toHaveBeenCalledOnce();
      expect(ai.run.mock.calls[0][0]).toBe(MODEL_ID);
      expect(r).toEqual({ is_guess: false, answer: "yes", hint: "long and tall" });
    });

    it("redacts secret leaking through hint", async () => {
      const ai = makeFakeAi();
      mockJudgement(ai, { is_guess: false, answer: "yes", hint: "it is an organ in a church" });
      const r = await judge({ AI: ai }, baseState(), "is it big?");
      expect(r.hint).not.toContain("organ");
      expect(r.hint).toContain("(redacted)");
    });

    it("wraps AI exception in UpstreamError", async () => {
      const ai = makeFakeAi();
      mockFailure(ai, new Error("network fail"));
      await expect(judge({ AI: ai }, baseState(), "is it big?")).rejects.toBeInstanceOf(
        UpstreamError,
      );
    });

    it("throws UpstreamError when env.AI missing", async () => {
      await expect(judge({}, baseState(), "is it big?")).rejects.toBeInstanceOf(UpstreamError);
    });

    it("uses default fallback when response is empty", async () => {
      const ai = makeFakeAi();
      ai.run.mockResolvedValueOnce({ response: "" });
      const r = await judge({ AI: ai }, baseState(), "is it big?");
      expect(r.is_guess).toBe(false);
      expect(r.answer).toBe("no");
    });

    it("does NOT send a tools array (drop function calling for Gemma compatibility)", async () => {
      const ai = makeFakeAi();
      mockJudgement(ai, { is_guess: false, answer: "yes", hint: "h" });
      await judge({ AI: ai }, baseState(), "is it big?");
      const [, body] = ai.run.mock.calls[0];
      expect(body.tools).toBeUndefined();
      expect(body.messages).toBeDefined();
    });
  });

  describe("generateRoundStart", () => {
    it("returns parsed { category, initialHint } on happy path", async () => {
      const ai = makeFakeAi();
      mockRoundStart(ai, { category: "instrument", initialHint: "cryptic clue about organs" });
      const r = await generateRoundStart({ AI: ai }, "organ");
      expect(r.category).toBe("instrument");
      expect(r.initialHint).toBe("cryptic clue about organs");
    });

    it("redacts the target word from the generated hint", async () => {
      const ai = makeFakeAi();
      mockRoundStart(ai, {
        category: "instrument",
        initialHint: "an organ is what you should think of",
      });
      const r = await generateRoundStart({ AI: ai }, "organ");
      expect(r.initialHint).not.toContain("organ");
    });

    it("falls back to safe defaults when model output is unparseable", async () => {
      const ai = makeFakeAi();
      ai.run.mockResolvedValueOnce({ response: "no json here" });
      const r = await generateRoundStart({ AI: ai }, "organ");
      expect(r.category).toBeTruthy();
      expect(r.initialHint).toBeTruthy();
    });

    it("falls back when model returns partial payload", async () => {
      const ai = makeFakeAi();
      ai.run.mockResolvedValueOnce({
        response: JSON.stringify({ category: "instrument" }),
      });
      const r = await generateRoundStart({ AI: ai }, "organ");
      expect(r.category).toBeTruthy();
      expect(r.initialHint).toBeTruthy();
    });

    it("wraps AI exception in UpstreamError", async () => {
      const ai = makeFakeAi();
      mockFailure(ai, new Error("boom"));
      await expect(generateRoundStart({ AI: ai }, "organ")).rejects.toBeInstanceOf(UpstreamError);
    });
  });
});
