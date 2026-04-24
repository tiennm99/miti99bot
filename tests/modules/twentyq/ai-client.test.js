import { describe, expect, it } from "vitest";
import {
  MODEL_ID,
  UpstreamError,
  judge,
  normalizeJudgement,
  parseToolCall,
  redactSecret,
} from "../../../src/modules/twentyq/ai-client.js";
import { makeFakeAi, mockFailure, mockJudgement } from "../../fakes/fake-ai.js";

const baseState = () => ({
  category: "instrument",
  target: "organ",
  initialHint: "uses wind through pipes",
  startedAt: 1,
  solved: false,
  turns: [],
});

describe("twentyq/ai-client", () => {
  describe("parseToolCall", () => {
    it("extracts traditional Cloudflare shape", () => {
      const r = parseToolCall({
        tool_calls: [
          { name: "submit_answer", arguments: { is_guess: false, answer: "yes", hint: "x" } },
        ],
      });
      expect(r).toEqual({ is_guess: false, answer: "yes", hint: "x" });
    });

    it("extracts OpenAI-style nested function shape", () => {
      const r = parseToolCall({
        tool_calls: [
          {
            function: {
              name: "submit_answer",
              arguments: { is_guess: true, answer: "no", hint: "y" },
            },
          },
        ],
      });
      expect(r).toEqual({ is_guess: true, answer: "no", hint: "y" });
    });

    it("parses stringified JSON arguments", () => {
      const r = parseToolCall({
        tool_calls: [
          {
            function: {
              name: "submit_answer",
              arguments: '{"is_guess":false,"answer":"no","hint":"z"}',
            },
          },
        ],
      });
      expect(r?.hint).toBe("z");
    });

    it("returns null when no tool_calls present", () => {
      expect(parseToolCall({})).toBeNull();
      expect(parseToolCall({ tool_calls: [] })).toBeNull();
      expect(parseToolCall(null)).toBeNull();
    });

    it("returns null on malformed stringified args", () => {
      const r = parseToolCall({
        tool_calls: [{ function: { name: "submit_answer", arguments: "not json" } }],
      });
      expect(r).toBeNull();
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

    it("uses default fallback when tool_calls absent", async () => {
      const ai = makeFakeAi();
      ai.run.mockResolvedValueOnce({}); // no tool_calls
      const r = await judge({ AI: ai }, baseState(), "is it big?");
      expect(r.is_guess).toBe(false);
      expect(r.answer).toBe("no");
    });
  });
});
