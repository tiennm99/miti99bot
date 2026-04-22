import { describe, expect, it, vi } from "vitest";
import { UpstreamError, createClient } from "../../../src/modules/semantle/api-client.js";

/**
 * Build a deterministic 384-dim vector (bge-small output size) from a seed
 * so cosine scores are reproducible without hardcoding 384 floats.
 */
function fakeVector(seed, dim = 384) {
  const out = new Array(dim);
  for (let i = 0; i < dim; i++) out[i] = Math.sin(seed * (i + 1));
  return out;
}

/**
 * Minimal Workers AI binding fake. `impl(model, input)` returns the payload
 * `env.AI.run()` would normally resolve to.
 */
function fakeAi(impl) {
  return { run: vi.fn(impl) };
}

describe("semantle/api-client", () => {
  describe("UpstreamError", () => {
    it("stores status and body metadata", () => {
      const err = new UpstreamError("test", { status: 404, body: "not found" });
      expect(err.message).toBe("test");
      expect(err.status).toBe(404);
      expect(err.body).toBe("not found");
      expect(err.name).toBe("UpstreamError");
    });

    it("stores cause when provided", () => {
      const cause = new Error("underlying");
      const err = new UpstreamError("wrapper", { cause });
      expect(err.cause).toBe(cause);
    });
  });

  describe("createClient", () => {
    it("throws without a valid AI binding", () => {
      expect(() => createClient(null)).toThrow(TypeError);
      expect(() => createClient({})).toThrow(TypeError);
      expect(() => createClient({ run: "not a function" })).toThrow(TypeError);
    });

    it("similarity batches target + guess in a single run() call", async () => {
      const ai = fakeAi(async (_model, { text }) => ({
        shape: [text.length, 384],
        data: text.map((_, i) => fakeVector(i + 1)),
      }));
      const client = createClient(ai);
      await client.similarity("apple", "orange");
      expect(ai.run).toHaveBeenCalledTimes(1);
      const [model, input] = ai.run.mock.calls[0];
      expect(model).toBe("@cf/baai/bge-small-en-v1.5");
      expect(input).toEqual({ text: ["apple", "orange"] });
    });

    it("similarity returns cosine score for in-vocab guess", async () => {
      const ai = fakeAi(async (_model, { text }) => ({
        data: text.map((_, i) => fakeVector(i + 1)),
      }));
      const client = createClient(ai);
      const res = await client.similarity("apple", "orange");
      expect(res.in_vocab_a).toBe(true);
      expect(res.in_vocab_b).toBe(true);
      expect(res.canonical_a).toBe("apple");
      expect(res.canonical_b).toBe("orange");
      expect(typeof res.similarity).toBe("number");
      expect(res.similarity).toBeGreaterThan(-1);
      expect(res.similarity).toBeLessThanOrEqual(1);
    });

    it("similarity returns 1 for identical vectors", async () => {
      const vec = fakeVector(7);
      const ai = fakeAi(async () => ({ data: [vec, vec] }));
      const client = createClient(ai);
      const res = await client.similarity("apple", "orange");
      expect(res.similarity).toBeCloseTo(1, 10);
    });

    it("similarity skips the AI call for OOV guess and flags in_vocab_b:false", async () => {
      const ai = fakeAi(async () => ({ data: [fakeVector(1), fakeVector(2)] }));
      const client = createClient(ai);
      const res = await client.similarity("apple", "zzzfoobarbaz");
      expect(res.in_vocab_b).toBe(false);
      expect(res.similarity).toBe(null);
      expect(ai.run).not.toHaveBeenCalled();
    });

    it("similarity wraps AI.run rejection as UpstreamError", async () => {
      const ai = fakeAi(async () => {
        throw new Error("boom");
      });
      const client = createClient(ai);
      await expect(client.similarity("apple", "orange")).rejects.toMatchObject({
        name: "UpstreamError",
      });
    });

    it("similarity throws UpstreamError on malformed payload", async () => {
      const ai = fakeAi(async () => ({ data: [fakeVector(1)] })); // only 1 vector
      const client = createClient(ai);
      await expect(client.similarity("apple", "orange")).rejects.toMatchObject({
        name: "UpstreamError",
      });
    });

    it("similarity returns null score when a vector norm is zero", async () => {
      const zero = new Array(384).fill(0);
      const ai = fakeAi(async () => ({ data: [zero, fakeVector(1)] }));
      const client = createClient(ai);
      const res = await client.similarity("apple", "orange");
      expect(res.in_vocab_b).toBe(true);
      expect(res.similarity).toBe(null);
    });

    it("randomWord returns a verified pick from the local pool", async () => {
      const ai = fakeAi(async () => ({ data: [] }));
      const client = createClient(ai);
      const res = await client.randomWord();
      expect(typeof res.word).toBe("string");
      expect(res.word.length).toBeGreaterThan(0);
      expect(res.verified).toBe(true);
      expect(ai.run).not.toHaveBeenCalled();
    });

    it("supports model override via options", async () => {
      const ai = fakeAi(async () => ({ data: [fakeVector(1), fakeVector(2)] }));
      const client = createClient(ai, { model: "@cf/baai/bge-large-en-v1.5" });
      await client.similarity("apple", "orange");
      expect(ai.run.mock.calls[0][0]).toBe("@cf/baai/bge-large-en-v1.5");
    });
  });
});
